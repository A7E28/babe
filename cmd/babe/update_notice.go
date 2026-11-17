package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/A7E28/babe/internal/tools"
)

type updateNoticeHandle struct {
	ch     <-chan string
	cancel context.CancelFunc
}

func isInteractiveTerminal() bool {
	return isCharDevice(os.Stdout) && isCharDevice(os.Stderr)
}

func isCharDevice(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func isTruthyEnv(val string) bool {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func shouldCheckForUpdates(args []string, curVersion string, isTTY bool, getenv func(string) string) bool {
	curVersion = strings.TrimSpace(curVersion)
	if curVersion == "" || curVersion == "dev" {
		return false
	}
	if !isTTY {
		return false
	}
	if hasJSONFlag(args) {
		return false
	}
	if getenv != nil {
		if isTruthyEnv(getenv("BABE_NO_UPDATE_CHECK")) || isTruthyEnv(getenv("CI")) {
			return false
		}
	}

	firstCmd := ""
	for _, raw := range args {
		if raw == "--" {
			break
		}
		if raw == "--version" || raw == "-h" || raw == "--help" {
			return false
		}
		if strings.HasPrefix(raw, "-") {
			continue
		}
		if firstCmd == "" {
			firstCmd = strings.ToLower(strings.TrimSpace(raw))
		}
	}

	switch firstCmd {
	case "update", "version", "help", "completion", "__complete", "__completeNoDesc", "__completeNoDesc:":
		return false
	}
	return true
}

func startBackgroundUpdateCheck(args []string) *updateNoticeHandle {
	if !shouldCheckForUpdates(args, version, isInteractiveTerminal(), os.Getenv) {
		return nil
	}

	cachePath, err := tools.UpdateCheckCachePath()
	if err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	ch := make(chan string, 1)

	go func() {
		defer close(ch)
		latest, available, err := tools.CheckForUpdateCached(ctx, version, cachePath, tools.DefaultUpdateCheckTTL)
		if err != nil || !available || latest == "" {
			return
		}
		ch <- fmt.Sprintf("Update available: %s -> %s. Run 'babe update self' to update.", version, latest)
	}()

	return &updateNoticeHandle{ch: ch, cancel: cancel}
}

func finishBackgroundUpdateCheck(handle *updateNoticeHandle, cmdSucceeded bool, w io.Writer) {
	if handle == nil {
		return
	}
	defer handle.cancel()

	if !cmdSucceeded || w == nil {
		return
	}

	timer := time.NewTimer(150 * time.Millisecond)
	defer timer.Stop()

	select {
	case msg, ok := <-handle.ch:
		if ok && msg != "" {
			fmt.Fprintln(w, msg)
		}
	case <-timer.C:
	}
}
