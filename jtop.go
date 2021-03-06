package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"strings"
	"syscall"
	"time"

	"github.com/nsf/termbox-go"
)

const (
	_  = iota
	KB = 1 << (10 * iota)
	MB
	GB
	TB
	PB
)

const usage = `Usage: jtop [options]

Options:
  -d, --delay    set delay between updates
  -k, --kernel   show kernel threads
  -p, --pids     filter by PID (comma-separated list)
  -s, --sort     sort by the specified column
  -t, --tree     display process list as tree
  -u, --users    filter by User (comma-separated list)
      --verbose  show full command line with arguments
`

var (
	delayFlag   time.Duration
	kernelFlag  bool
	pidsFlag    string
	sortFlag    string
	treeFlag    bool
	usersFlag   string
	verboseFlag bool
)

func exitf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "jtop: "+format+"\n", a...)
	os.Exit(1)
}

func signalSelf(sig syscall.Signal) {
	if err := syscall.Kill(os.Getpid(), sig); err != nil {
		panic(err)
	}
}

func validateDelayFlag() {
	if delayFlag <= 0 {
		exitf("delay (%s) must be positive", delayFlag)
	}
}

func validatePidsFlag() {
	if pidsFlag == "" {
		return
	}

	pids := strings.Split(pidsFlag, ",")
	for _, value := range pids {
		if pid, err := ParseUint64(value); err != nil {
			exitf("%s is not a valid PID", value)
		} else {
			PidWhitelist = append(PidWhitelist, pid)
		}
	}
}

func validateSortFlag() {
	for _, column := range Columns {
		if sortFlag == column.Title {
			return
		}
	}
	exitf("%s is not a valid sort column", sortFlag)
}

func validateUsersFlag() {
	if usersFlag == "" {
		return
	}

	users := strings.Split(usersFlag, ",")
	for _, username := range users {
		if user, err := user.Lookup(username); err != nil {
			exitf("user %s does not exist", username)
		} else {
			UserWhitelist = append(UserWhitelist, user)
		}
	}
}

func validateFlags() {
	validateDelayFlag()
	validatePidsFlag()
	validateSortFlag()
	validateUsersFlag()
}

func init() {
	defaultDelay := time.Duration(1500 * time.Millisecond)
	flag.DurationVar(&delayFlag, "d", defaultDelay, "")
	flag.DurationVar(&delayFlag, "delay", defaultDelay, "")

	flag.BoolVar(&kernelFlag, "k", false, "")
	flag.BoolVar(&kernelFlag, "kernel", false, "")

	flag.StringVar(&pidsFlag, "p", "", "")
	flag.StringVar(&pidsFlag, "pids", "", "")

	defaultSort := CPUPercentColumn.Title
	flag.StringVar(&sortFlag, "s", defaultSort, "")
	flag.StringVar(&sortFlag, "sort", defaultSort, "")

	flag.BoolVar(&treeFlag, "t", false, "")
	flag.BoolVar(&treeFlag, "tree", false, "")

	flag.StringVar(&usersFlag, "u", "", "")
	flag.StringVar(&usersFlag, "users", "", "")

	flag.BoolVar(&verboseFlag, "verbose", false, "")

	flag.Usage = func() {
		fmt.Fprint(os.Stdout, usage)
	}
}

func termboxInit() {
	if err := termbox.Init(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func main() {
	flag.Parse()
	validateFlags()

	termboxInit()
	defer termbox.Close()

	events := make(chan termbox.Event)
	go func() {
		for {
			events <- termbox.PollEvent()
		}
	}()

	ticker := time.Tick(delayFlag)
	monitor := NewMonitor()
	monitor.Update()
	ui := NewUI(monitor)

	for {
		ui.Draw()

		select {
		case <-ticker:
			monitor.Update()

		case ev := <-events:
			if ev.Type == termbox.EventKey {
				switch {
				case ev.Ch == 'q' || ev.Key == termbox.KeyCtrlC:
					return
				case ev.Ch == 'h' || ev.Key == termbox.KeyArrowLeft:
					ui.HandleLeft()
				case ev.Ch == 'j' || ev.Key == termbox.KeyArrowDown:
					ui.HandleDown()
				case ev.Ch == 'k' || ev.Key == termbox.KeyArrowUp:
					ui.HandleUp()
				case ev.Ch == 'l' || ev.Key == termbox.KeyArrowRight:
					ui.HandleRight()
				case ev.Ch == '0' || ev.Ch == '^':
					ui.HandleResetOffset()
				case ev.Ch == 'g':
					ui.HandleSelectFirst()
				case ev.Ch == 'G':
					ui.HandleSelectLast()
				case ev.Ch == 't':
					treeFlag = !treeFlag
					monitor.Update()
				case ev.Ch == 'v':
					verboseFlag = !verboseFlag
				case ev.Key == termbox.KeyCtrlD:
					ui.HandleCtrlD()
				case ev.Key == termbox.KeyCtrlU:
					ui.HandleCtrlU()
				case ev.Key == termbox.KeyCtrlZ:
					termbox.Close()
					signalSelf(syscall.SIGTSTP)
					termboxInit()
				}
			} else if ev.Type == termbox.EventResize {
				ui.HandleResize(ev.Width, ev.Height)
			}
		}
	}
}
