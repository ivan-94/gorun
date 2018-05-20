package gorun

import (
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// Runner 运行器
type Runner struct {
	args []string
	mux  sync.Mutex
	cmd  *exec.Cmd
}

// Run 运行
func (r *Runner) Run() {
	r.mux.Lock()
	defer r.mux.Unlock()
	cmd := exec.Command("go", r.args...)
	cmdStr := "go " + strings.Join(r.args, " ")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
	r.cmd = cmd

	log.Printf("running: `%s`\n", cmdStr)
	go cmd.Run()
}

// Kill 杀死进程
func (r *Runner) Kill() {
	defer func() {
		if e := recover(); e != nil {
			Printf("kill recover")
		}
	}()
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.cmd != nil && r.cmd.Process != nil {
		pid := r.cmd.Process.Pid
		Printf("killing %d", pid)
		if runtime.GOOS == "windows" {
			// TODO: test
			// kill the pid tree
			cmd := exec.Command("taskkill", "/pid", strconv.Itoa(pid), "/T", "/F")
			err := cmd.Run()
			if err != nil {
				log.Printf("failed to kill %d with taskkill in Windows: %s", pid, err)
				return
			}
		} else {
			// kill the progress group
			pgid, err := syscall.Getpgid(pid)
			if err != nil {
				log.Printf("failed to get pgid of %d: %s", pid, err)
				return
			}
			err = syscall.Kill(-pgid, syscall.SIGKILL)
			if err != nil {
				Printf("failed to kill -%d: %s", pgid, err)
				return
			}
		}
	}
}

// Restart 重启
func (r *Runner) Restart() {
	r.Kill()
	r.Run()
}

// NewRunner 运行器构造函数
func NewRunner(args []string) *Runner {
	return &Runner{
		args: append([]string{"run"}, args...),
	}
}
