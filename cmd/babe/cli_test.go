package main

import (
	"bufio"
	"reflect"
	"strings"
	"testing"
)

func TestReorderFlags(t *testing.T) {
	got := reorderFlags(
		[]string{"app.apk", "-s", "R58M1234", "-3"},
		map[string]bool{"-3": true},
	)
	want := []string{"-s", "R58M1234", "-3", "app.apk"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("reorderFlags() = %v, want %v", got, want)
	}
}

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

func TestIsHelpArg(t *testing.T) {
	cases := []struct {
		arg  string
		want bool
	}{
		{"-h", true},
		{"--help", true},
		{"help", true},
		{"-help", true},
		{"status", false},
		{"list", false},
		{"-v", false},
		{"--version", false},
	}
	for _, tc := range cases {
		if got := isHelpArg(tc.arg); got != tc.want {
			t.Errorf("isHelpArg(%q) = %v, want %v", tc.arg, got, tc.want)
		}
	}
}

func TestHelpSubcommands_DoNotPanic(t *testing.T) {
	cmds := [][]string{
		{},
		{"doctor"},
		{"tools"},
		{"update"},
		{"setup"},
		{"device"},
		{"device", "info"},
		{"device", "reboot"},
		{"fastboot"},
		{"fastboot", "slot"},
		{"fastboot", "flash"},
		{"apk"},
		{"apk", "inspect"},
		{"apk", "install"},
		{"apk", "list"},
		{"apk", "pull"},
		{"image"},
		{"logcat"},
		{"logcat", "crash"},
		{"hash"},
		{"unknown-subcommand"},
	}
	for _, c := range cmds {
		handleHelpCommand(c)
	}
}
