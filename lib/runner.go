package gorun

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

// Runner 运行器
type Runner struct {
	args []string
	cmd  *exec.Cmd
}

// Run 运行
func (r *Runner) Run() {
	cmd := exec.Command("go", r.args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	r.cmd = cmd

	log.Printf("running: `%s`\n", "go "+strings.Join(r.args, " "))

	go cmd.Run()
}

// Kill 杀死进程
func (r *Runner) Kill() {
	defer func() {
		if e := recover(); e != nil {
			Printf("kill recover")
		}
	}()
	if r.cmd != nil && r.cmd.Process != nil {
		err := r.cmd.Process.Kill()
		if err != nil {
			Printf("error when kill `go run`: %s\n", err)
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
