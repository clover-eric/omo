package singbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const defaultTimeout = 3 * time.Second

var versionPattern = regexp.MustCompile(`sing-box\s+version\s+([^\s]+)`)

type Options struct {
	BinaryPath     string
	CandidatePaths []string
	Timeout        time.Duration
}

type Status struct {
	Installed     bool   `json:"installed"`
	Healthy       bool   `json:"healthy"`
	Version       string `json:"version,omitempty"`
	Path          string `json:"path,omitempty"`
	InstallSource string `json:"installSource,omitempty"`
	Message       string `json:"message"`
}

type Detector struct {
	options Options
	runner  commandRunner
	look    func(string) (string, error)
	stat    func(string) (os.FileInfo, error)
}

type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type execRunner struct{}

func NewDetector(options Options) *Detector {
	return &Detector{
		options: options,
		runner:  execRunner{},
		look:    exec.LookPath,
		stat:    os.Stat,
	}
}

func (d *Detector) Status(ctx context.Context) (Status, error) {
	if d == nil {
		d = NewDetector(Options{})
	}
	binaryPath, source, err := d.resolveBinary()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return Status{
				Installed: false,
				Healthy:   false,
				Message:   "未检测到 sing-box 接入核心，请先完成核心安装。",
			}, nil
		}
		return Status{}, err
	}

	timeout := d.options.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	versionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := d.runner.Run(versionCtx, binaryPath, "version")
	if err != nil {
		return Status{
			Installed:     true,
			Healthy:       false,
			Path:          binaryPath,
			InstallSource: source,
			Message:       "已检测到 sing-box，但版本检查失败，请确认二进制可执行。",
		}, nil
	}

	version := ParseVersion(string(output))
	if version == "" {
		return Status{
			Installed:     true,
			Healthy:       false,
			Path:          binaryPath,
			InstallSource: source,
			Message:       "已检测到 sing-box，但无法识别版本输出。",
		}, nil
	}

	return Status{
		Installed:     true,
		Healthy:       true,
		Version:       version,
		Path:          binaryPath,
		InstallSource: source,
		Message:       "sing-box 接入核心已安装并可执行。",
	}, nil
}

func (d *Detector) resolveBinary() (string, string, error) {
	if d.options.BinaryPath != "" {
		path := filepath.Clean(d.options.BinaryPath)
		if _, err := d.stat(path); err != nil {
			return "", "", err
		}
		return path, "configured_path", nil
	}

	if path, err := d.look("sing-box"); err == nil && path != "" {
		return path, "path", nil
	}

	for _, candidate := range d.candidatePaths() {
		path := filepath.Clean(candidate)
		if _, err := d.stat(path); err == nil {
			return path, "candidate_path", nil
		}
	}

	return "", "", exec.ErrNotFound
}

func (d *Detector) candidatePaths() []string {
	if len(d.options.CandidatePaths) > 0 {
		return d.options.CandidatePaths
	}
	return []string{
		"/usr/local/bin/sing-box",
		"/usr/bin/sing-box",
		"/opt/omo/bin/sing-box",
	}
}

func ParseVersion(output string) string {
	match := versionPattern.FindStringSubmatch(strings.TrimSpace(output))
	if len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func (execRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return output, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return output, err
	}
	return output, nil
}
