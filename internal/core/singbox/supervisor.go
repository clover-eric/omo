package singbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Supervisor struct {
	BinaryPath string

	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan error
}

func NewSupervisor(binaryPath string) *Supervisor {
	return &Supervisor{BinaryPath: strings.TrimSpace(binaryPath)}
}

func (s *Supervisor) Reload(ctx context.Context, configPath string) error {
	if s == nil {
		return nil
	}
	if strings.TrimSpace(configPath) == "" {
		return errors.New("sing-box config path is empty")
	}
	if _, err := os.Stat(configPath); err != nil {
		return err
	}
	binary := strings.TrimSpace(s.BinaryPath)
	if binary == "" {
		resolved, err := exec.LookPath("sing-box")
		if err != nil {
			return err
		}
		binary = resolved
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.stopLocked(); err != nil {
		return err
	}
	cmd := exec.CommandContext(context.Background(), binary, "run", "-c", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start sing-box: %w", err)
	}
	s.cmd = cmd
	exited := make(chan error, 1)
	s.done = exited
	go func() {
		err := cmd.Wait()
		slog.Info("sing-box access core exited", "error", err)
		exited <- err
	}()

	select {
	case err := <-exited:
		s.cmd = nil
		s.done = nil
		if err != nil {
			return fmt.Errorf("sing-box exited during startup: %w", err)
		}
		return errors.New("sing-box exited during startup")
	case <-time.After(700 * time.Millisecond):
		return nil
	case <-ctx.Done():
		_ = s.stopLocked()
		return ctx.Err()
	}
}

func (s *Supervisor) Stop() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopLocked()
}

func (s *Supervisor) stopLocked() error {
	if s.cmd == nil || s.cmd.Process == nil {
		s.cmd = nil
		return nil
	}
	process := s.cmd.Process
	done := s.done
	_ = process.Signal(syscall.SIGTERM)
	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = process.Kill()
			<-done
		}
	} else {
		time.Sleep(100 * time.Millisecond)
	}
	s.cmd = nil
	s.done = nil
	return nil
}
