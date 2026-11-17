package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestShouldCheckForUpdates(t *testing.T) {
	noEnv := func(string) string { return "" }
	cases := []struct {
		name    string
		args    []string
		version string
		isTTY   bool
		env     map[string]string
		want    bool
	}{
		{"normal tty command", []string{"devices"}, "v0.4.0", true, nil, true},
		{"dev build suppressed", []string{"devices"}, "dev", true, nil, false},
		{"non-tty suppressed", []string{"devices"}, "v0.4.0", false, nil, false},
		{"json flag suppressed", []string{"devices", "--json"}, "v0.4.0", true, nil, false},
		{"update cmd suppressed", []string{"update", "self"}, "v0.4.0", true, nil, false},
		{"version cmd suppressed", []string{"version"}, "v0.4.0", true, nil, false},
		{"version flag suppressed", []string{"--version"}, "v0.4.0", true, nil, false},
		{"help cmd suppressed", []string{"help", "doctor"}, "v0.4.0", true, nil, false},
		{"help flag suppressed", []string{"doctor", "--help"}, "v0.4.0", true, nil, false},
		{"completion suppressed", []string{"__complete", "dev"}, "v0.4.0", true, nil, false},
		{"BABE_NO_UPDATE_CHECK=1", []string{"devices"}, "v0.4.0", true, map[string]string{"BABE_NO_UPDATE_CHECK": "1"}, false},
		{"CI=true", []string{"devices"}, "v0.4.0", true, map[string]string{"CI": "true"}, false},
		{"CI=1", []string{"devices"}, "v0.4.0", true, map[string]string{"CI": "1"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getenv := noEnv
			if tc.env != nil {
				getenv = func(k string) string { return tc.env[k] }
			}
			if got := shouldCheckForUpdates(tc.args, tc.version, tc.isTTY, getenv); got != tc.want {
				t.Errorf("shouldCheckForUpdates(%v, %q, tty=%v) = %v, want %v",
					tc.args, tc.version, tc.isTTY, got, tc.want)
			}
		})
	}
}

func TestFinishBackgroundUpdateCheck(t *testing.T) {
	ch := make(chan string, 1)
	ch <- "Update available: v0.4.0 -> v0.5.0. Run 'babe update self' to update."
	close(ch)

	_, cancel := context.WithCancel(context.Background())
	var buf bytes.Buffer
	finishBackgroundUpdateCheck(&updateNoticeHandle{ch: ch, cancel: cancel}, true, &buf)
	if !strings.Contains(buf.String(), "Update available: v0.4.0 -> v0.5.0") {
		t.Errorf("expected notice to be printed, got %q", buf.String())
	}

	// Failed command suppresses notice
	ch2 := make(chan string, 1)
	ch2 <- "Update available: v0.4.0 -> v0.5.0. Run 'babe update self' to update."
	close(ch2)
	_, cancel2 := context.WithCancel(context.Background())
	var buf2 bytes.Buffer
	finishBackgroundUpdateCheck(&updateNoticeHandle{ch: ch2, cancel: cancel2}, false, &buf2)
	if buf2.Len() != 0 {
		t.Errorf("expected suppressed notice on failed command, got %q", buf2.String())
	}
}
