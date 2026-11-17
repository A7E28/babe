package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/platform"
)

func TestCLI_RootVersionFlagJSON(t *testing.T) {
	out, errOut, err := runCLIWithBuffers("--version", "--json")
	if err != nil {
		t.Fatalf("--version --json failed: %v (stderr: %s)", err, errOut)
	}
	if errOut != "" {
		t.Errorf("expected empty stderr, got %q", errOut)
	}

	var got versionReport
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", out, err)
	}
	if got.Version != version {
		t.Errorf("got version %q, want %q", got.Version, version)
	}
}

func TestCLI_SubcommandsRequireTargetUnderJSON(t *testing.T) {
	cases := [][]string{
		{"--json"},
		{"image", "--json"},
		{"update", "--json"},
		{"update", "bogus", "--json"},
		{"update", "--tools", "--self", "--json"},
		{"update", "--tools", "tools", "--json"},
		{"apk", "--json"},
		{"fastboot", "--json"},
	}

	for _, args := range cases {
		out, errOut, err := runCLIWithBuffers(args...)
		if err == nil {
			t.Errorf("expected error for %v, got nil", args)
			continue
		}
		if out != "" {
			t.Errorf("expected empty stdout for %v, got %q", args, out)
		}
		var errGot errorReport
		if jErr := json.Unmarshal([]byte(errOut), &errGot); jErr != nil {
			t.Errorf("expected JSON error on stderr for %v, got %q: %v", args, errOut, jErr)
			continue
		}
		if errGot.Error == "" {
			t.Errorf("expected non-empty error field for %v", args)
		}
	}
}

func TestHasJSONFlag_EdgeCases(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"doctor", "--json", "--json=false"}, false},
		{[]string{"doctor", "--json=false", "--json"}, true},
		{[]string{"doctor", "--json=1"}, true},
		{[]string{"doctor", "--json=0"}, false},
		{[]string{"doctor", "--json=invalid"}, false},
		{[]string{"doctor", "--json", "--", "--json=false"}, true},
		{[]string{"hash", "--", "--json"}, false},
	}

	for _, tc := range tests {
		if got := hasJSONFlag(tc.args); got != tc.want {
			t.Errorf("hasJSONFlag(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}

	out, errOut, err := runCLIWithBuffers("hash", "--json", "--json=false")
	if err == nil {
		t.Fatalf("expected error for missing hash arg")
	}
	if out != "" {
		t.Errorf("expected empty stdout, got %q", out)
	}
	if !strings.Contains(errOut, "[ERROR]") {
		t.Errorf("expected non-JSON [ERROR] on stderr when --json=false overrides --json, got %q", errOut)
	}
}

type failingPathPM struct {
	binDir string
}

func (f *failingPathPM) GetDefaultInstallDir() (string, error) { return f.binDir, nil }
func (f *failingPathPM) GetBabeBinDir() (string, error)        { return f.binDir, nil }
func (f *failingPathPM) IsInPath(dir string) (bool, error)     { return false, nil }
func (f *failingPathPM) AddToPath(dir string) error            { return errors.New("registry denied") }
func (f *failingPathPM) RemoveFromPath(dir string) error       { return nil }
func (f *failingPathPM) OpenFilePicker(title string) (string, error) {
	return "", nil
}

func TestCLI_SetupPartialSuccessReportsPath(t *testing.T) {
	pm := &failingPathPM{binDir: t.TempDir()}
	cmd := newRootCmd(device.NewClient(), fastboot.NewClient(), pm)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err := executeRootCmd(cmd, []string{"setup", "--json"})
	if err == nil {
		t.Fatalf("expected setup error when AddToPath fails")
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout on error, got %q", stdout.String())
	}
	var errGot errorReport
	if jErr := json.Unmarshal(stderr.Bytes(), &errGot); jErr != nil {
		t.Fatalf("expected JSON error on stderr, got %q: %v", stderr.String(), jErr)
	}
	if !strings.Contains(errGot.Error, "installed babe binary at") {
		t.Errorf("expected partial-success context in error, got %q", errGot.Error)
	}

	pm2 := &failingPathPM{binDir: t.TempDir()}
	cmdNonJSON := newRootCmd(device.NewClient(), fastboot.NewClient(), pm2)
	var out2, err2 bytes.Buffer
	cmdNonJSON.SetOut(&out2)
	cmdNonJSON.SetErr(&err2)
	if err := executeRootCmd(cmdNonJSON, []string{"setup"}); err == nil {
		t.Fatalf("expected non-JSON setup to return error when AddToPath fails")
	}
	if !strings.Contains(err2.String(), "[ERROR]") || !strings.Contains(err2.String(), "installed babe binary at") {
		t.Errorf("expected [ERROR] and partial-success context on stderr, got %q", err2.String())
	}
	var _ platform.PlatformManager = pm
}
