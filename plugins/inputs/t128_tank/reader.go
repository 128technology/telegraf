package t128_tank

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
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
	sequenceStr        = []byte("seq=")
	colonStr           = []byte(":")
)

type boundaryFault struct {
	nextAvailableIndex index
}

func (bf *boundaryFault) Error() string {
	return fmt.Sprintf("next available index is %d", bf.nextAvailableIndex.value)
}

type IndexedMessage struct {
	Message []byte
	Index   index
}

type index struct {
	value uint64
}

func newIndex(data string) (index, error) {
	data = strings.TrimSpace(data)
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
	topic             string
	lastObservedIndex chan uint64
	// used to send events from the read routine to the send routine
	sendChan chan []IndexedMessage
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
	// telegraf Logger
	log telegraf.Logger
}

func NewReader(tankAddress string, tankPort int, topic string, indexPath string, defaultIndex index, log telegraf.Logger) *Reader {
	return &Reader{
		topic:             topic,
		lastObservedIndex: make(chan uint64, 1),
		tankReadCmdCtx:    exec.CommandContext,
		sendChan:          make(chan []IndexedMessage),
		readDone:          make(chan error),
		restartDelay:      5 * time.Second,
		tankAddress:       tankAddress,
		tankPort:          tankPort,
		indexPath:         indexPath,
		defaultIndex:      defaultIndex,
		log:               log,
	}
}

func (r *Reader) withTankReadCommandContext(tankReadCmdCtx CommandContext) *Reader {
	r.tankReadCmdCtx = tankReadCmdCtx
	return r
}

func (r *Reader) Run(mainCtx context.Context) {
	readCtx, readCtxCancel := context.WithCancel(mainCtx)
	var observedValue uint64
	lastIndex, err := r.getIndex(r.indexPath, r.defaultIndex)
	if err != nil {
		r.log.Errorf("Error in get index %v", err)
		readCtxCancel()
		return
	}

	var wg sync.WaitGroup
	nextSaveCheck := time.NewTicker(2 * time.Second)
	defer func() {
		readCtxCancel()
		go func() {
			wg.Wait()
			close(r.lastObservedIndex)
		}()

		var anyFound bool
		var lastObservedValue uint64
	drainLoop:
		for {
			select {
			case observedValue, ok := <-r.lastObservedIndex:
				if !ok {
					break drainLoop
				}
				anyFound = true
				lastObservedValue = observedValue
			case <-r.readDone:
			}
		}
		if anyFound {
			r.setIndex(r.indexPath, index{value: lastObservedValue})
		} else {
			r.setIndex(r.indexPath, index{value: lastIndex.value})
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		r.read(readCtx, lastIndex.next())
	}()

	for {
		select {
		case <-mainCtx.Done():
			r.log.Errorf("%s reader done", r.topic)
			return

		case val := <-r.lastObservedIndex:
			if val > lastIndex.value {
				lastIndex.value = val
				if lastIndex.value%1000 == 0 {
					r.setIndex(r.indexPath, index{value: lastIndex.value})
				}
			}

		case <-nextSaveCheck.C:
			if observedValue > lastIndex.value {
				lastIndex.value = observedValue
				r.setIndex(r.indexPath, index{value: lastIndex.value})
			}

		case err := <-r.readDone:
			var errBoundaryFault *boundaryFault
			if err != nil && errors.As(err, &errBoundaryFault) {
				r.log.Debugf("detected boundary fault, restarting %s tank read from index %d", r.topic, errBoundaryFault.nextAvailableIndex.value)
				lastIndex = errBoundaryFault.nextAvailableIndex
				wg.Add(1)
				go func() {
					defer wg.Done()
					r.read(readCtx, lastIndex)
				}()
			} else {
				if observedValue > lastIndex.value {
					lastIndex.value = observedValue
				}
				lastIndex = lastIndex.next()
				wg.Add(1)
				go func() {
					defer wg.Done()
					r.read(readCtx, lastIndex)
				}()
			}
		}
	}
}

func (r *Reader) getIndex(indexPath string, defaultIndex index) (index, error) {
	if indexPath == "" {
		r.log.Debugf("index file path not provided, starting with default index %s", defaultIndex.string())
		return defaultIndex, nil
	}
	content, err := os.ReadFile(indexPath)
	if errors.Is(err, os.ErrNotExist) {
		r.log.Debugf("index file %s does not exist, starting with default index %s", indexPath, defaultIndex.string())
		return defaultIndex, nil

	} else if err != nil {
		return defaultIndex, fmt.Errorf("encountered error reading index file, starting with default index %s: %s", defaultIndex.string(), err)
	}
	newContent := strings.Split(string(content), "\n")[0]
	r.log.Debugf("found '%s' in index file", newContent)
	index, err := newIndex(newContent)
	if err != nil {
		return defaultIndex, fmt.Errorf("encountered error while parsing index file content, starting with default index %s: %s", defaultIndex.string(), err)
	}

	return index, nil
}

func (r *Reader) setIndex(indexPath string, index index) {
	if indexPath == "" {
		r.log.Debugf("index file path not provided, starting with index %d", index.value)
		return
	}
	_, err := os.Stat(path.Dir(indexPath))
	if errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(path.Dir(indexPath), os.ModePerm)
		if err != nil {
			r.log.Debugf("unable to create index file parent directory: %s", err)
			return
		}
	}

	err = os.WriteFile(indexPath, []byte(index.string()), 0600)
	if err != nil {
		r.log.Debugf("unable to update index to %d: %s", index, err)
	}
}

func (r *Reader) read(readCtx context.Context, startingIndex index) {
	r.log.Infof("starting %s tank read from index %d", r.topic, startingIndex.value)
	err := r.readFromTank(readCtx, startingIndex)
	if err != nil {
		r.log.Errorf("%s read routine exited with error: %v", r.topic, err)
	}

	if readCtx.Err() == nil {
		time.Sleep(r.restartDelay)
	}

	r.readDone <- err
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

	cmdContext, cmdCancel := context.WithCancel(readCtx)

	cmd := r.tankReadCmdCtx(cmdContext, cmdArgs[0], cmdArgs[1:]...)

	defer func() {
		cmdCancel()
		if cmdErr := cmd.Wait(); cmdErr != nil {
			var exitErr *exec.ExitError
			if errors.As(cmdErr, &exitErr) {
				r.log.Errorf("%s tank read command exited with error: %s", r.topic, exitErr.Error())
			} else {
				r.log.Errorf("%s tank read command exited with error: %s", r.topic, cmdErr)
			}
		}
	}()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to pipe %s tank output: %s", r.topic, err)
	}

	tankReader := bufio.NewReader(stdout)
	if err := cmd.Start(); err != nil {
		r.log.Errorf("Error starting command: %s", err.Error())
		return fmt.Errorf("unable to start %s tank stream: %w", r.topic, err)
	}

	r.log.Infof("Command started successfully")

	var parseErr error
	var messages []*IndexedMessage
parseLoop:
	for {
		select {
		case <-cmdContext.Done():
			r.log.Errorf("%s read routine done", r.topic)
			break parseLoop
		default:
		}

		messages, parseErr = parseLines(tankReader, r.topic)
		if parseErr != nil {
			r.log.Errorf("encountered error while parsing lines in %s output, will not attempt to parse more lines: %v", r.topic, parseErr)
			return parseErr
		}
		var collectedMessages []IndexedMessage
		for _, message := range messages {
			collectedMessages = append(collectedMessages, *message)
			r.lastObservedIndex <- message.Index.value
		}
		select {
		case <-cmdContext.Done():
			break parseLoop
		case r.sendChan <- collectedMessages:
		}
	}

	return parseErr
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
			fmt.Errorf("parse failure for topic %s: %v", topic, err)
			continue
		}

		if message == nil {
			if isBoundaryFault, nextAvailableIndex := isBoundaryFault(line); isBoundaryFault {
				return messages, fmt.Errorf(
					"parsed boundary fault: %w",
					&boundaryFault{nextAvailableIndex: nextAvailableIndex},
				)
			}

			fmt.Errorf("skipping %s line because regex matching failed: %s", topic, line)
			continue
		}
		if message != nil && message.IsValid() {
			message.Message = bytes.TrimRight(message.Message, "\n")
			messages = append(messages, message)
		} else {
			return nil, fmt.Errorf("invalid message received : %s", line)
		}

	}

	return messages, err
}

func ParseTankLine(line []byte) (*IndexedMessage, error) {
	if !bytes.HasPrefix(line, sequenceStr) {
		return nil, nil
	}

	colonIdx := bytes.Index(line, colonStr)
	if colonIdx < 5 {
		return nil, nil
	}

	indexBytes := line[4:colonIdx]

	index, err := newIndex(string(indexBytes))
	if err != nil {
		return nil, fmt.Errorf("unable to parse index %s, skipping: %s", indexBytes, err)
	}

	return &IndexedMessage{
		Message: line[colonIdx+1:],
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
		fmt.Errorf("unable to parse index after detecting boundary fault: %s", err)
		return false, StartIndex
	}

	return true, nextIndex
}
