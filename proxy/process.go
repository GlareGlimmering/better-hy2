package proxy

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"h-ui/model/constant"
	"h-ui/util"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

var logger logrus.Logger

func initLogger() {
	logger.SetOutput(util.NewSynchronizedWriter(constant.Hysteria2LogPath, &lumberjack.Logger{
		Filename:   constant.Hysteria2LogPath,
		MaxSize:    1,
		MaxBackups: 2,
		MaxAge:     30,
		Compress:   true,
		LocalTime:  true,
	}))
	logger.SetFormatter(&logrus.JSONFormatter{TimestampFormat: "2006-01-02 15:04:05"})
	logger.SetLevel(logrus.InfoLevel)
}

func init() {
	initLogger()
}

type process struct {
	mutex        *sync.Mutex
	cmd          *exec.Cmd
	outputMutex  sync.RWMutex
	recentOutput []string
	startupReady bool
}

const recentProcessOutputLimit = 32

func (p *process) resetOutput() {
	p.outputMutex.Lock()
	p.recentOutput = nil
	p.startupReady = false
	p.outputMutex.Unlock()
}

func (p *process) recordOutput(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if len(line) > 4096 {
		line = line[:4096]
	}
	p.outputMutex.Lock()
	if strings.Contains(line, "server up and running") {
		p.startupReady = true
	}
	p.recentOutput = append(p.recentOutput, line)
	if len(p.recentOutput) > recentProcessOutputLimit {
		p.recentOutput = append([]string(nil), p.recentOutput[len(p.recentOutput)-recentProcessOutputLimit:]...)
	}
	p.outputMutex.Unlock()
}

func (p *process) mainServerReady() bool {
	p.outputMutex.RLock()
	defer p.outputMutex.RUnlock()
	return p.startupReady
}

func (p *process) lastOutput() string {
	p.outputMutex.RLock()
	defer p.outputMutex.RUnlock()
	if len(p.recentOutput) == 0 {
		return ""
	}
	return p.recentOutput[len(p.recentOutput)-1]
}

func (p *process) isRunning() bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	running, _ := p.refreshRunningLocked()
	return running
}

// refreshRunningLocked reconciles the in-memory child handle with the actual
// OS process. exec.Cmd.ProcessState is only populated after Wait, so checking
// it alone leaves an asynchronously exited HY2 child looking alive forever.
func (p *process) refreshRunningLocked() (bool, error) {
	if p.cmd == nil || p.cmd.Process == nil || p.cmd.ProcessState != nil {
		return false, nil
	}

	if runtime.GOOS == "linux" {
		stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", p.cmd.Process.Pid))
		if os.IsNotExist(err) {
			return false, p.reapExitedLocked()
		}
		if err == nil {
			closing := strings.LastIndexByte(string(stat), ')')
			if closing >= 0 && len(stat) > closing+2 {
				state := stat[closing+2]
				if state == 'Z' || state == 'X' {
					return false, p.reapExitedLocked()
				}
			}
		}
		if err := p.cmd.Process.Signal(syscall.Signal(0)); err != nil {
			return false, p.reapExitedLocked()
		}
	}
	return true, nil
}

func (p *process) reapExitedLocked() error {
	if p.cmd == nil {
		return nil
	}
	err := p.cmd.Wait()
	p.cmd = nil
	return err
}

func (p *process) start(name string, arg ...string) error {
	return p.startWithEnv(name, nil, arg...)
}

func (p *process) startWithEnv(name string, env []string, arg ...string) error {
	if !p.mutex.TryLock() {
		logrus.Errorf("start cmd err: lock not acquired")
		return errors.New("start cmd err")
	}
	defer p.mutex.Unlock()

	if running, _ := p.refreshRunningLocked(); running {
		return nil
	}
	p.resetOutput()

	cmd := exec.Command(name, arg...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}
	if cmd.Err != nil {
		logrus.Errorf("cmd err: %v", cmd.Err)
		return errors.New("cmd err")
	}

	// 获取命令的 stdout 和 stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logrus.Errorf("Error obtaining stdout: %v", err)
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logrus.Errorf("Error obtaining stderr: %v", err)
		return err
	}

	if err := cmd.Start(); err != nil {
		logrus.Errorf("cmd start err: %v", err)
		return errors.New("cmd start err")
	}

	p.cmd = cmd

	go p.handleLogs(stdout, stderr)
	time.Sleep(150 * time.Millisecond)
	if running, exitErr := p.refreshRunningLocked(); !running {
		output := p.lastOutput()
		if exitErr != nil {
			if output != "" {
				return fmt.Errorf("process exited during startup: %s: %w", output, exitErr)
			}
			return fmt.Errorf("process exited during startup: %w", exitErr)
		}
		if output != "" {
			return fmt.Errorf("process exited during startup: %s", output)
		}
		return errors.New("process exited during startup")
	}

	return nil
}

func (p *process) stop() error {
	if !p.mutex.TryLock() {
		return errors.New("cmd stop err: lock not acquired")
	}

	running, _ := p.refreshRunningLocked()
	if !running {
		p.mutex.Unlock()
		return nil
	}

	cmd := p.cmd
	p.mutex.Unlock()

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		logrus.Warnf("send SIGTERM failed: %v", err)
	}

	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()

	select {
	case err := <-done:
		if normalizeExitErr(err, syscall.SIGTERM) != nil {
			return fmt.Errorf("process exit failed: %w", err)
		}

	case <-timer.C:
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("SIGKILL failed: %w", err)
		}

		err := <-done
		if normalizeExitErr(err, syscall.SIGKILL) != nil {
			return fmt.Errorf("process killed but exit abnormal: %w", err)
		}
	}

	p.mutex.Lock()
	p.cmd = nil
	p.mutex.Unlock()

	return nil
}

func normalizeExitErr(err error, allowedSignals ...syscall.Signal) error {
	if err == nil {
		return nil
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return err
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return err
	}

	if status.Signaled() {
		sig := status.Signal()
		for _, s := range allowedSignals {
			if sig == s {
				return nil
			}
		}
	}

	return err
}

func (p *process) release() error {
	if !p.mutex.TryLock() {
		logrus.Errorf("cmd release err: lock not acquired")
		return errors.New("cmd release err")
	}
	defer p.mutex.Unlock()

	running, _ := p.refreshRunningLocked()
	if !running {
		return nil
	}

	if err := p.cmd.Process.Release(); err != nil {
		logrus.Errorf("cmd release err: %v", err)
		return errors.New("cmd release err")
	}
	p.cmd = nil
	return nil
}

func (p *process) handleLogs(stdout, stderr io.ReadCloser) {
	// 日志
	stdoutChan := make(chan string)
	stderrChan := make(chan string)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			stdoutChan <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			logrus.Errorf("Error reading stdout: %v", err)
		}
		close(stdoutChan)
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			stderrChan <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			logrus.Errorf("Error reading stderr: %v", err)
		}
		close(stderrChan)
	}()

	for {
		select {
		case line, ok := <-stdoutChan:
			if !ok {
				stdoutChan = nil
			} else {
				p.recordOutput(line)
				hysteria2AccessLogger.consume(line)
				logger.Info(line)
			}
		case line, ok := <-stderrChan:
			if !ok {
				stderrChan = nil
			} else {
				p.recordOutput(line)
				hysteria2AccessLogger.consume(line)
				logger.Error(line)
			}
		}

		// 当两个 channel 都关闭时，退出循环
		if stdoutChan == nil && stderrChan == nil {
			break
		}
	}
}
