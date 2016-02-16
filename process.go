package main

import (
	"bufio"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type ProcessType int

const (
	ProcessUser ProcessType = iota
	ProcessKernel
)

// Process represents an operating system process.
type Process struct {
	Pid     int
	User    *user.User
	Command string
	Type    ProcessType
}

func NewProcess(pid int) Process {
	command := cmdline(pid)

	user := userFromPid(pid)

	pt := ProcessUser
	if command == "" {
		pt = ProcessKernel
	}

	return Process{
		Pid:     pid,
		User:    user,
		Command: command,
		Type:    pt,
	}
}

// ByPid implements sort.Interface for []Process based on the Pid field.
type ByPid []Process

func (p ByPid) Len() int           { return len(p) }
func (p ByPid) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p ByPid) Less(i, j int) bool { return p[i].Pid < p[j].Pid }

func getRunningProcesses() []Process {
	files, err := ioutil.ReadDir("/proc")
	if err != nil {
		panic(err)
	}

	var processes []Process

	for _, file := range files {
		if !file.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(file.Name())
		if err != nil {
			continue // non-PID directory
		}

		p := NewProcess(pid)
		if p.Type != ProcessKernel {
			processes = append(processes, p)
		}
	}

	sort.Sort(ByPid(processes))
	return processes
}

// cmdline returns the command used to start `pid`.
func cmdline(pid int) string {
	path := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")

	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	s := string(data)

	// Sometimes the arguments are separated by NUL as well as ending in multiple
	// trailing NULs. Fix that so we return something that looks like you'd type
	// in the shell.
	return strings.TrimSpace(strings.Replace(s, "\x00", " ", -1))
}

// userFromPid returns the effective user running process `pid`.
func userFromPid(pid int) *user.User {
	path := filepath.Join("/proc", strconv.Itoa(pid), "status")

	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var uid string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}

		//       R     E     SS    FS
		// Uid:\t1000\t1000\t1000\t1000
		pieces := strings.Split(line, "\t")

		uid = pieces[2]
		break
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}

	user, err := userByUid(uid)
	if err != nil {
		panic(err)
	}

	return user
}
