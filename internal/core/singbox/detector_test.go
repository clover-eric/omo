package singbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeRunner struct {
	output []byte
	err    error
	name   string
	args   []string
}

func (r *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.name = name
	r.args = args
	return r.output, r.err
}

func TestParseVersion(t *testing.T) {
	version := ParseVersion("sing-box version 1.12.8\nEnvironment: go1.25\n")
	if version != "1.12.8" {
		t.Fatalf("expected version 1.12.8, got %q", version)
	}
}

func TestStatusDetectsConfiguredBinary(t *testing.T) {
	binaryPath := filepath.Join("opt", "omo", "bin", "sing-box")
	runner := &fakeRunner{output: []byte("sing-box version 1.12.8\n")}
	detector := NewDetector(Options{BinaryPath: binaryPath})
	detector.runner = runner
	detector.stat = func(path string) (os.FileInfo, error) {
		if path != binaryPath {
			t.Fatalf("unexpected stat path %q", path)
		}
		return fakeFileInfo{}, nil
	}

	status, err := detector.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Installed || !status.Healthy || status.Version != "1.12.8" {
		t.Fatalf("expected healthy installed status, got %#v", status)
	}
	if runner.name != binaryPath || len(runner.args) != 1 || runner.args[0] != "version" {
		t.Fatalf("expected version command, name=%q args=%v", runner.name, runner.args)
	}
}

func TestStatusReportsMissingBinary(t *testing.T) {
	detector := NewDetector(Options{CandidatePaths: []string{"/missing/sing-box"}})
	detector.look = func(string) (string, error) { return "", errors.New("not in path") }
	detector.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }

	status, err := detector.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Installed || status.Healthy {
		t.Fatalf("expected missing status, got %#v", status)
	}
}

func TestStatusReportsUnhealthyVersionCommand(t *testing.T) {
	detector := NewDetector(Options{BinaryPath: "/usr/local/bin/sing-box"})
	detector.runner = &fakeRunner{err: errors.New("permission denied")}
	detector.stat = func(string) (os.FileInfo, error) { return fakeFileInfo{}, nil }

	status, err := detector.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Installed || status.Healthy {
		t.Fatalf("expected unhealthy installed status, got %#v", status)
	}
}

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "sing-box" }
func (fakeFileInfo) Size() int64        { return 1 }
func (fakeFileInfo) Mode() os.FileMode  { return 0o755 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() any           { return nil }
