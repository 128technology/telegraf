package t128_tank

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	endIndexUint   = math.MaxUint64
	endIndexStr    = "end"
	startIndexUint = 0
	startIndexStr  = "0"
)

var (
	StartIndex         = index{value: startIndexUint}
	EndIndex           = index{value: endIndexUint}
	boundaryFaultRegex = regexp.MustCompile(`^Boundary Check fault. first available sequence number is (\d+), .*$`)
	indexRegex         = regexp.MustCompile(`seq=(\d+?):(.*)$`)
)

type boundaryFault struct {
	nextAvailableIndex index
}

func (bf *boundaryFault) Error() string {
	return fmt.Sprintf("next available index is %d", bf.nextAvailableIndex.value)
}

// InfluxLine
type IndexedMessage struct {
	Message   []byte
	Index     index
	Timestamp int
}

type index struct {
	value uint64
}

func newIndex(data string) (index, error) {
	if string(data) == endIndexStr {
		return EndIndex, nil
	}
	indexVal, err := strconv.ParseUint(data, 10, 64)
	if err != nil {
		return StartIndex, fmt.Errorf("unable to parse %s as integer: %s", data, err)
	}
	return index{value: indexVal}, nil
}

func (i index) string() string {
	if i.value == endIndexUint {
		return endIndexStr
	}
	return fmt.Sprintf("%d", i.value)
}

func (i index) next() index {
	if i.value == startIndexUint || i.value == endIndexUint {
		return i
	}
	return index{value: i.value + 1}
}

type CommandContext = func(ctx context.Context, name string, arg ...string) *exec.Cmd

type Reader struct {
	topic          string
	lastSavedIndex uint64
	// used to send events from the read routine to the send routine
	sendChan chan IndexedMessage
	// the target address for the TANK instance
	tankAddress string
	// the target port for the TANK instance
	tankPort int
	// time to wait before restarting the tank read command
	restartDelay time.Duration
	// used to notify the main routine that the read routine finished with an error
	readDone chan error
	// used to mock tank read subprocess in testing
	tankReadCmdCtx CommandContext
	// path to the index file
	indexPath string
	// default index in case reading to/from the index file fails
	defaultIndex index
}

func NewReader(tankAddress string, tankPort int, topic string, indexPath string, defaultIndex index) *Reader {
	return &Reader{
		topic:          topic,
		tankReadCmdCtx: exec.CommandContext,
		sendChan:       make(chan IndexedMessage),
		readDone:       make(chan error),
		restartDelay:   5 * time.Second,
		tankAddress:    tankAddress,
		tankPort:       tankPort,
		indexPath:      indexPath,
		defaultIndex:   defaultIndex,
	}
}

func (r *Reader) withTankReadCommandContext(tankReadCmdCtx CommandContext) *Reader {
	r.tankReadCmdCtx = tankReadCmdCtx
	return r
}

func (r *Reader) Run(mainCtx context.Context, stopchan chan struct{}) {
	readCtx, readCtxCancel := context.WithCancel(mainCtx)
	paused := false
	lastIndex, err := getIndex(r.indexPath, r.defaultIndex)
	r.lastSavedIndex = lastIndex.value
	if err != nil {
		log.Printf("Error in get index")
		return
	}

	nextSaveCheck := time.After(2 * time.Second)
	defer func() {
		readCtxCancel()
		setIndex(r.indexPath, index{value: r.lastSavedIndex})
	}()

	go r.read(mainCtx, readCtx, lastIndex.next())

	for {
		select {
		case <-mainCtx.Done():
			fmt.Printf("%s reader done", r.topic)
			readCtxCancel()
			return
		case <-nextSaveCheck:
			if lastIndex.value > r.lastSavedIndex {
				setIndex(r.indexPath, lastIndex)
				r.lastSavedIndex = lastIndex.value
			}
			nextSaveCheck = time.After(2 * time.Second)
		case err := <-r.readDone:
			if !paused {
				readCtx, readCtxCancel = context.WithCancel(mainCtx)
				var boundaryFault *boundaryFault
				if err != nil && errors.As(err, &boundaryFault) {
					fmt.Printf("detected boundary fault, restarting %s tank read from index %d", r.topic, boundaryFault.nextAvailableIndex.value)
					go r.read(mainCtx, readCtx, boundaryFault.nextAvailableIndex)
				} else {
					go r.read(mainCtx, readCtx, lastIndex.next())
				}
			} else {
				fmt.Printf("reader is paused, skipping %s tank read restart", r.topic)
			}
		}
	}
}

func (r *Reader) read(mainCtx context.Context, readCtx context.Context, startingIndex index) {
	fmt.Printf("starting %s tank read from index %d", r.topic, startingIndex.value)
	err := r.readFromTank(readCtx, startingIndex)
	if err != nil {
		fmt.Printf("%s read routine exited with error: %v", r.topic, err)
	}

	if mainCtx.Err() == nil {
		time.Sleep(r.restartDelay)
	}

	r.readDone <- err
}

func (i *IndexedMessage) WithTimestamp(timestamp int) []byte {
	parts := strings.Fields(string(i.Message))
	parts[len(parts)-1] = strconv.Itoa(timestamp)
	return []byte(strings.Join(parts, " "))
}

func (i *IndexedMessage) IsValid() bool {
	return len(i.Message) > 0
}

func parseLines(tankReader *bufio.Reader, topic string) (messages []*IndexedMessage, err error) {
	messages = make([]*IndexedMessage, 0, 1)
	rawLines, err := tankReader.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return messages, err
	} else if len(rawLines) == 0 {
		return messages, fmt.Errorf("found empty newline in %s tank stream, continuing", topic)
	}
	rawLines = bytes.TrimSpace(rawLines)
	lines := bytes.Split(rawLines, []byte("\n"))
	for _, rawLine := range lines {
		line := string(rawLine)

		if len(line) == 0 {
			continue
		}
		message, err := ParseTankLine(rawLine)
		if err != nil {
			fmt.Printf("parse failure for topic %s: %v", topic, err)
			continue
		}

		if message == nil {
			if isBoundaryFault, nextAvailabelIndex := isBoundaryFault(line); isBoundaryFault {
				return messages, fmt.Errorf(
					"parsed boundary fault: %w",
					&boundaryFault{nextAvailableIndex: nextAvailabelIndex},
				)
			}

			fmt.Printf("skipping %s line because regex matching failed: %s", topic, line)
			continue
		}

		message.Message = bytes.TrimRight(message.Message, "\n")
		messages = append(messages, message)
	}

	return messages, err
}

func ParseTankLine(line []byte) (*IndexedMessage, error) {
	matches := indexRegex.FindSubmatch(line)
	if len(matches) != 3 {
		return nil, errors.New("invalid line format")
	}

	index, err := newIndex(string(matches[1]))
	if err != nil {
		return nil, fmt.Errorf("unable to parse index %s, skipping: %s", matches[1], err)
	}

	message := matches[2]
	return &IndexedMessage{
		Message: message,
		Index:   index,
	}, nil
}

func isBoundaryFault(line string) (bool, index) {
	matches := boundaryFaultRegex.FindStringSubmatch(line)
	if len(matches) != 2 {
		return false, StartIndex
	}

	nextIndex, err := newIndex(matches[1])
	if err != nil {
		fmt.Printf("unable to parse index after detecting boundary fault: %s", err)
		return false, StartIndex
	}

	return true, nextIndex
}

func (r *Reader) readFromTank(readCtx context.Context, startingIndex index) (err error) {
	cmdArgs := []string{
		"/opt/128technology/bin/tank-cli",
		"-b",
		fmt.Sprintf("%s:%d", r.tankAddress, r.tankPort),
		"-t",
		r.topic,
		"consume",
		"-F",
		"seqnum,content",
		startingIndex.string(),
	}

	cmd := r.tankReadCmdCtx(readCtx, cmdArgs[0], cmdArgs[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to pipe %s tank output: %s", r.topic, err)
	}

	tankReader := bufio.NewReader(stdout)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("unable to start %s tank stream: %w", r.topic, err)
	}

	var parseErr error
	var messages []*IndexedMessage

	for {
		select {
		case <-readCtx.Done():
			fmt.Printf("%s read routine done", r.topic)
			return nil
		default:
		}

		messages, parseErr = parseLines(tankReader, r.topic)
		if parseErr != nil {
			fmt.Printf("encountered error while parsing lines in %s output, will not attempt to parse more lines: %v", r.topic, parseErr)
			return parseErr
		}
		fmt.Println("After Parse Line inside read from tank")
		for _, message := range messages {
			if message.IsValid() {
				select {
				case <-readCtx.Done():
					return nil
				case r.sendChan <- *message:
				}
			}
		}
	}
}

func getIndex(indexPath string, defaultIndex index) (index, error) {
	content, err := os.ReadFile(indexPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("index file %s does not exist, starting with default index %s", indexPath, defaultIndex.string())
		return defaultIndex, nil

	} else if err != nil {
		return defaultIndex, fmt.Errorf("encountered error reading index file, starting with default index %s: %s", defaultIndex.string(), err)
	}

	log.Printf("found '%s' in index file", content)

	index, err := newIndex(string(content))
	if err != nil {
		return defaultIndex, fmt.Errorf("encountered error while parsing index file content, starting with default index %s: %s", defaultIndex.string(), err)
	}

	return index, nil
}

func setIndex(indexPath string, index index) {
	_, err := os.Stat(path.Dir(indexPath))
	if errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(path.Dir(indexPath), os.ModePerm)
		if err != nil {
			fmt.Printf("unable to create index file parent directory: %s", err)
			return
		}
	}

	err = os.WriteFile(indexPath, []byte(index.string()), 0644)
	if err != nil {
		fmt.Printf("unable to update index to %d: %s", index, err)
	}
}
