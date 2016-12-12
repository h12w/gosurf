package gosurf

import (
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"h12.me/errors"
	"h12.me/uuid"
)

type surf struct {
	id  string
	cmd *exec.Cmd
}

// it is not necessary to pass the exact request because the request is sent back to our proxy
// so basically any uri is OK
func startSurf(uri, proxy string) (*surf, error) {
	// id is for debugging only
	id, _ := uuid.NewTime(time.Now())
	cmd := exec.Command(
		"surf",
		"-bdfgikmnp",
		"-t", os.DevNull,
		uri,
		id.String(),
	)
	// set pgid so all child processes can be killed together
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = []string{
		"DISPLAY=" + os.Getenv("DISPLAY"),
		"http_proxy=" + proxy,
	}
	return &surf{id: id.String(), cmd: cmd}, cmd.Start()
}

func (b *surf) pid() int {
	if b.cmd.Process != nil {
		return b.cmd.Process.Pid
	}
	return 0
}

func (b *surf) Wait() error {
	err := b.cmd.Wait()
	if _, ok := err.(*exec.ExitError); !ok {
		return errors.Wrap(err)
	}
	return nil
}

func (b *surf) Close() error {
	if b.cmd.Process == nil {
		log.Printf("cannot kill surf %s because it is not started", b.id)
		return nil
	}

	// kill -pgid (-pid)
	// https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773#.g2krdc3ir
	if err := syscall.Kill(-b.cmd.Process.Pid, syscall.SIGKILL); err != nil {
		log.Printf("fail to kill surf %s (%d)", b.id, b.pid())
		return err
	}
	return nil
}

func forceKill(p *os.Process) error {
	if err := p.Kill(); err != nil {
		return err
	}
	for i := 0; processExists(p.Pid); i++ {
		if err := p.Kill(); err != nil {
			return err
		}
		time.Sleep(time.Second)
		if i > 10 {
			log.Printf("try to kill surf %d for the %d times", p.Pid, i)
		}
	}
	return nil
}

func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		// non-unix system
		return false
	}
	return nil == process.Signal(syscall.Signal(0))
}

func errChan(f func() error) chan error {
	ch := make(chan error)
	go func() {
		ch <- f()
	}()
	return ch
}
