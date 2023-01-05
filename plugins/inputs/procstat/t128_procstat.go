package procstat

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/influxdata/telegraf"
)

type T128Procstat struct {
	Procstat

	PidFinder              string `toml:"pid_finder"`
	PidFile                string `toml:"pid_file"`
	Exe                    string
	Pattern                string
	Prefix                 string
	CmdLineTag             bool `toml:"cmdline_tag"`
	ProcessName            string
	User                   string
	SystemdUnit            string `toml:"systemd_unit"`
	IncludeSystemdChildren bool   `toml:"include_systemd_children"`
	CGroup                 string `toml:"cgroup"`
	PidTag                 bool
	WinService             string `toml:"win_service"`
	Mode                   string

	solarisMode bool

	finder PIDFinder

	createPIDFinder func() (PIDFinder, error)
	procs           map[PID]Process
	createProcess   func(PID) (Process, error)
}

var t128sampleConfig = `
  ## PID file to monitor process
  pid_file = "/var/run/nginx.pid"
  ## executable name (ie, pgrep <exe>)
  # exe = "nginx"
  ## pattern as argument for pgrep (ie, pgrep -f <pattern>)
  # pattern = "nginx"
  ## user as argument for pgrep (ie, pgrep -u <user>)
  # user = "nginx"
  ## Systemd unit name, supports globs when include_systemd_children is set to true
  # systemd_unit = "nginx.service"
  # include_systemd_children = false
  ## CGroup name or path, supports globs
  # cgroup = "systemd/system.slice/nginx.service"

  ## Windows service name
  # win_service = ""

  ## override for process_name
  ## This is optional; default is sourced from /proc/<pid>/status
  # process_name = "bar"

  ## Field name prefix
  # prefix = ""

  ## When true add the full cmdline as a tag.
  # cmdline_tag = false

  ## Mode to use when calculating CPU usage. Can be one of 'solaris' or 'irix'.
  # mode = "irix"

  ## Add the PID as a tag instead of as a field.  When collecting multiple
  ## processes with otherwise matching tags this setting should be enabled to
  ## ensure each process has a unique identity.
  ##
  ## Enabling this option may result in a large number of series, especially
  ## when processes have a short lifetime.
  # pid_tag = false

  ## Method to use when finding process IDs.  Can be one of 'pgrep', or
  ## 'native'.  The pgrep finder calls the pgrep executable in the PATH while
  ## the native finder performs the search directly in a manor dependent on the
  ## platform.  Default is 'pgrep'
  # pid_finder = "pgrep"
`

func (p *T128Procstat) SampleConfig() string {
	return t128sampleConfig
}

func (p *T128Procstat) Description() string {
	return "Monitor process cpu and memory usage by 128T"
}

// Add metrics a single Process
func (p *T128Procstat) addMetric(proc Process, acc telegraf.Accumulator, t time.Time) {
	var prefix string
	if p.Prefix != "" {
		prefix = p.Prefix + "_"
	}

	fields := map[string]interface{}{}

	//If process_name tag is not already set, set to actual name
	if _, nameInTags := proc.Tags()["process_name"]; !nameInTags {
		name, err := proc.Name()
		if err == nil {
			proc.Tags()["process_name"] = name
		}
	}

	//If user tag is not already set, set to actual name
	if _, ok := proc.Tags()["user"]; !ok {
		user, err := proc.Username()
		if err == nil {
			proc.Tags()["user"] = user
		}
	}

	//If pid is not present as a tag, include it as a field.
	if _, pidInTags := proc.Tags()["pid"]; !pidInTags {
		fields["pid"] = int32(proc.PID())
	}

	//If cmd_line tag is true and it is not already set add cmdline as a tag
	if p.CmdLineTag {
		if _, ok := proc.Tags()["cmdline"]; !ok {
			cmdline, err := proc.Cmdline()
			if err == nil {
				proc.Tags()["cmdline"] = cmdline
			}
		}
	}

	createdAt, err := proc.CreateTime() //Returns epoch in ms
	if err == nil {
		fields[prefix+"created_at"] = createdAt * 1000000 //Convert ms to ns
	}

	cpuTime, err := proc.Times()
	if err == nil {
		fields[prefix+"cpu_time_user"] = cpuTime.User
		fields[prefix+"cpu_time_system"] = cpuTime.System
		fields[prefix+"cpu_time_idle"] = cpuTime.Idle
		fields[prefix+"cpu_time_nice"] = cpuTime.Nice
		fields[prefix+"cpu_time_iowait"] = cpuTime.Iowait
		fields[prefix+"cpu_time_irq"] = cpuTime.Irq
		fields[prefix+"cpu_time_soft_irq"] = cpuTime.Softirq
		fields[prefix+"cpu_time_steal"] = cpuTime.Steal
		fields[prefix+"cpu_time_guest"] = cpuTime.Guest
		fields[prefix+"cpu_time_guest_nice"] = cpuTime.GuestNice
	}

	cpuPerc, err := proc.Percent(time.Duration(0))
	if err == nil {
		if p.solarisMode {
			fields[prefix+"cpu_usage"] = cpuPerc / float64(runtime.NumCPU())
		} else {
			fields[prefix+"cpu_usage"] = cpuPerc
		}
	}

	mem, err := proc.MemoryInfo()
	if err == nil {
		fields[prefix+"memory_rss"] = mem.RSS
		fields[prefix+"memory_vms"] = mem.VMS
		fields[prefix+"memory_swap"] = mem.Swap
		fields[prefix+"memory_data"] = mem.Data
		fields[prefix+"memory_stack"] = mem.Stack
		fields[prefix+"memory_locked"] = mem.Locked
	}

	ppid, err := proc.Ppid()
	if err == nil {
		fields[prefix+"ppid"] = ppid
	}

	acc.AddFields("procstat", fields, proc.Tags(), t)
}

// Update monitored Processes
// func (p *T128Procstat) updateProcesses(pids []PID, tags map[string]string, prevInfo map[PID]Process, procs map[PID]Process) error {
// 	for _, pid := range pids {
// 		info, ok := prevInfo[pid]
// 		if ok {
// 			// Assumption: if a process has no name, it probably does not exist
// 			if name, _ := info.Name(); name == "" {
// 				continue
// 			}
// 			procs[pid] = info
// 		} else {
// 			proc, err := p.createProcess(pid)
// 			if err != nil {
// 				// No problem; process may have ended after we found it
// 				continue
// 			}
// 			// Assumption: if a process has no name, it probably does not exist
// 			if name, _ := proc.Name(); name == "" {
// 				continue
// 			}
// 			procs[pid] = proc

// 			// Add initial tags
// 			for k, v := range tags {
// 				proc.Tags()[k] = v
// 			}

// 			// Add pid tag if needed
// 			if p.PidTag {
// 				proc.Tags()["pid"] = strconv.Itoa(int(pid))
// 			}
// 			if p.ProcessName != "" {
// 				proc.Tags()["process_name"] = p.ProcessName
// 			}
// 		}
// 	}
// 	return nil
// }

// execCommand is so tests can mock out exec.Command usage.
var t128execCommand = exec.Command

func (p *T128Procstat) simpleSystemdUnitPIDs() ([]PID, error) {
	var pids []PID

	cmd := t128execCommand("systemctl", "show", p.SystemdUnit)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		kv := bytes.SplitN(line, []byte{'='}, 2)
		if len(kv) != 2 {
			continue
		}
		if !bytes.Equal(kv[0], []byte("MainPID")) {
			continue
		}
		if len(kv[1]) == 0 || bytes.Equal(kv[1], []byte("0")) {
			return nil, nil
		}
		pid, err := strconv.ParseInt(string(kv[1]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid pid '%s'", kv[1])
		}
		pids = append(pids, PID(pid))
	}

	return pids, nil
}

// func (p *T128Procstat) cgroupPIDs() []PidsTags {
// 	var pidTags []PidsTags

// 	procsPath := p.CGroup
// 	if procsPath[0] != '/' {
// 		procsPath = "/sys/fs/cgroup/" + procsPath
// 	}
// 	items, err := filepath.Glob(procsPath)
// 	if err != nil {
// 		pidTags = append(pidTags, PidsTags{nil, nil, fmt.Errorf("glob failed '%s'", err)})
// 		return pidTags
// 	}
// 	for _, item := range items {
// 		pids, err := p.singleCgroupPIDs(item)
// 		tags := map[string]string{"cgroup": p.CGroup, "cgroup_full": item}
// 		pidTags = append(pidTags, PidsTags{pids, tags, err})
// 	}

// 	return pidTags
// }

// func (p *T128Procstat) singleCgroupPIDs(path string) ([]PID, error) {
// 	var pids []PID

// 	ok, err := isDir(path)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if !ok {
// 		return nil, fmt.Errorf("not a directory %s", path)
// 	}
// 	procsPath := filepath.Join(path, "cgroup.procs")
// 	out, err := os.ReadFile(procsPath)
// 	if err != nil {
// 		return nil, err
// 	}
// 	for _, pidBS := range bytes.Split(out, []byte{'\n'}) {
// 		if len(pidBS) == 0 {
// 			continue
// 		}
// 		pid, err := strconv.ParseInt(string(pidBS), 10, 32)
// 		if err != nil {
// 			return nil, fmt.Errorf("invalid pid '%s'", pidBS)
// 		}
// 		pids = append(pids, PID(pid))
// 	}

// 	return pids, nil
// }

// func (p *T128Procstat) winServicePIDs() ([]PID, error) {
// 	var pids []PID

// 	pid, err := queryPidWithWinServiceName(p.WinService)
// 	if err != nil {
// 		return pids, err
// 	}

// 	pids = append(pids, PID(pid))

// 	return pids, nil
// }

// func (p *T128Procstat) Init() error {
// 	if strings.ToLower(p.Mode) == "solaris" {
// 		p.solarisMode = true
// 	}

// 	return nil
// }
