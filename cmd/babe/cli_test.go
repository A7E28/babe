package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/platform"
)

func TestReadLine_HandlesEOF(t *testing.T) {
	emptyReader := bufio.NewReader(strings.NewReader(""))
	if line, ok := readLine(emptyReader); ok || line != "" {
		t.Errorf("expected (\"\", false) on empty EOF reader, got (%q, %v)", line, ok)
	}

	noNewlineEOF := bufio.NewReader(strings.NewReader("  y  "))
	if line, ok := readLine(noNewlineEOF); !ok || line != "y" {
		t.Errorf("expected (\"y\", true) on trailing data before EOF, got (%q, %v)", line, ok)
	}
	if line, ok := readLine(noNewlineEOF); ok || line != "" {
		t.Errorf("expected (\"\", false) on subsequent read after EOF, got (%q, %v)", line, ok)
	}
}

func TestCLI_HelpSubcommands_DoNotPanic(t *testing.T) {
	adbClient := device.NewClient()
	fbClient := fastboot.NewClient()
	pm := platform.DefaultManager()

	cases := [][]string{
		{"--help"},
		{"help"},
		{"doctor", "--help"},
		{"tools", "--help"},
		{"tools", "status", "--help"},
		{"tools", "update", "--help"},
		{"tools", "setup", "--help"},
		{"update", "--help"},
		{"setup", "--help"},
		{"device", "--help"},
		{"device", "list", "--help"},
		{"device", "info", "--help"},
		{"device", "reboot", "--help"},
		{"fastboot", "--help"},
		{"fastboot", "devices", "--help"},
		{"fastboot", "info", "--help"},
		{"fastboot", "slot", "--help"},
		{"fastboot", "flash", "--help"},
		{"apk", "--help"},
		{"apk", "inspect", "--help"},
		{"apk", "install", "--help"},
		{"apk", "list", "--help"},
		{"apk", "pull", "--help"},
		{"image", "--help"},
		{"image", "inspect", "--help"},
		{"logcat", "--help"},
		{"logcat", "crash", "--help"},
		{"hash", "--help"},
		{"udev", "--help"},
		{"version"},
		{"devices", "--help"},
		{"info", "--help"},
		{"reboot", "--help"},
		{"packages", "--help"},
		{"pull-apk", "--help"},
		{"install", "--help"},
	}

	for _, args := range cases {
		cmd := newRootCmd(adbClient, fbClient, pm)
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs(args)

		if err := cmd.Execute(); err != nil {
			t.Errorf("cmd.Execute(%v) failed: %v", args, err)
		}
		if buf.Len() == 0 {
			t.Errorf("cmd.Execute(%v) produced empty output", args)
		}
	}
}

func TestCLI_POSIXFlagOrdering(t *testing.T) {
	adbClient := device.NewClient()
	fbClient := fastboot.NewClient()
	pm := platform.DefaultManager()

	// Ensure flags can be placed after positional arguments without error
	cmd := newRootCmd(adbClient, fbClient, pm)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Testing help flag after subcommand
	cmd.SetArgs([]string{"apk", "inspect", "test.apk", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error parsing flags after args: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "inspect") {
		t.Errorf("expected help output to mention inspect, got:\n%s", out)
	}
}

func TestCLI_VersionCommand(t *testing.T) {
	adbClient := device.NewClient()
	fbClient := fastboot.NewClient()
	pm := platform.DefaultManager()

	cmd := newRootCmd(adbClient, fbClient, pm)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version flag failed: %v", err)
	}

	if !strings.Contains(buf.String(), "babe") {
		t.Errorf("expected version output to contain 'babe', got: %q", buf.String())
	}
}

func TestCLI_ArgValidationsAndHelp(t *testing.T) {
	invalidCases := [][]string{
		{"update", "bogus"},
		{"update", "--tools", "--self"},
		{"update", "--tools", "tools"},
		{"update", "tools", "extra"},
		{"update", "self", "extra"},
		{"doctor", "extra"},
		{"setup", "extra"},
		{"tools", "extra"},
		{"device", "extra"},
		{"info", "extra"},
		{"image", "a.img", "b.img"},
		{"install", "a.apk", "extra"},
	}
	for _, args := range invalidCases {
		out, errOut, err := runCLIWithBuffers(args...)
		if err == nil {
			t.Errorf("expected error for %v, got nil", args)
		}
		if out != "" {
			t.Errorf("expected empty stdout on error for %v, got %q", args, out)
		}
		if !strings.Contains(errOut, "[ERROR]") {
			t.Errorf("expected [ERROR] on stderr for %v, got %q", args, errOut)
		}
	}

	helpCases := [][]string{
		{},
		{"image"},
		{"update"},
		{"apk"},
		{"fastboot"},
	}
	for _, args := range helpCases {
		out, errOut, err := runCLIWithBuffers(args...)
		if err != nil {
			t.Errorf("expected nil error for help command %v, got %v (stderr: %s)", args, err, errOut)
		}
		if out == "" {
			t.Errorf("expected non-empty help output for %v", args)
		}
	}
}
