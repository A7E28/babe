package logcat

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// StreamOptions configures the logcat streaming session.
type StreamOptions struct {
	ADBBinary   string
	Serial      string
	MinLevel    Level
	TagFilter   string
	PIDFilter   string
	PackageName string
	ClearBefore bool
	OutputFile  string
}

type LineHandler func(entry Entry)

func isDigitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Stream connects to `adb logcat`, streams entries in real-time, and terminates cleanly when ctx is cancelled.
func Stream(ctx context.Context, opts StreamOptions, handler LineHandler) error {
	adbBin := opts.ADBBinary
	if adbBin == "" {
		adbBin = "adb"
	}

	if opts.ClearBefore {
		clearArgs := serialArgs(opts.Serial, "logcat", "-c")
		if out, err := exec.CommandContext(ctx, adbBin, clearArgs...).CombinedOutput(); err != nil && ctx.Err() == nil {
			msg := strings.TrimSpace(string(out))
			if msg != "" {
				return fmt.Errorf("adb logcat -c failed (%w): %s", err, msg)
			}
			return fmt.Errorf("adb logcat -c failed: %w", err)
		}
	}

	args := serialArgs(opts.Serial, "logcat", "-b", "main,system", "-v", "time")
	if opts.MinLevel != LevelVerbose {
		args = append(args, fmt.Sprintf("*:%s", opts.MinLevel.String()))
	}

	pids, err := resolveTargetPIDs(ctx, adbBin, opts)
	if err != nil {
		return err
	}
	for _, pid := range pids {
		args = append(args, "--pid="+pid)
	}

	tagLower := strings.ToLower(opts.TagFilter)
	_, err = runLogcatProcess(ctx, adbBin, args, opts.OutputFile, func(entry Entry) bool {
		if tagLower != "" && !strings.Contains(strings.ToLower(entry.Tag), tagLower) {
			return false
		}
		if entry.Level < opts.MinLevel {
			return false
		}
		if handler != nil {
			handler(entry)
		}
		return true
	})
	return err
}

// DumpCrashLogs harvests the device's crash ring buffer (`logcat -b crash -v time -d`) and exits immediately.
func DumpCrashLogs(ctx context.Context, adbBinary, serial, outputFile string, handler LineHandler) (int, error) {
	if adbBinary == "" {
		adbBinary = "adb"
	}
	args := serialArgs(serial, "logcat", "-b", "crash", "-v", "time", "-d")
	return runLogcatProcess(ctx, adbBinary, args, outputFile, func(entry Entry) bool {
		if handler != nil {
			handler(entry)
		}
		return true
	})
}

func serialArgs(serial string, tail ...string) []string {
	var args []string
	if serial != "" {
		args = append(args, "-s", serial)
	}
	return append(args, tail...)
}

func resolveTargetPIDs(ctx context.Context, adbBin string, opts StreamOptions) ([]string, error) {
	if opts.PIDFilter != "" {
		return []string{opts.PIDFilter}, nil
	}
	if opts.PackageName == "" {
		return nil, nil
	}
	if isDigitsOnly(opts.PackageName) {
		return []string{opts.PackageName}, nil
	}

	pidCmdArgs := serialArgs(opts.Serial, "shell", "pidof", opts.PackageName)
	out, err := exec.CommandContext(ctx, adbBin, pidCmdArgs...).Output()
	if err != nil {
		return nil, fmt.Errorf("process %q is not running on device", opts.PackageName)
	}

	var pids []string
	for _, p := range strings.Fields(string(out)) {
		if isDigitsOnly(p) {
			pids = append(pids, p)
		}
	}
	if len(pids) == 0 {
		return nil, fmt.Errorf("process %q is not running on device", opts.PackageName)
	}
	return pids, nil
}

func runLogcatProcess(ctx context.Context, adbBin string, args []string, outputFile string, onEntry func(Entry) bool) (int, error) {
	cmd := exec.CommandContext(ctx, adbBin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("open logcat stdout pipe: %w", err)
	}

	var fileWriter io.WriteCloser
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return 0, fmt.Errorf("create logcat output file: %w", err)
		}
		fileWriter = f
		defer fileWriter.Close()
	}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start logcat subprocess: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 512*1024)

	count := 0
	for scanner.Scan() {
		raw := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(raw), "--------- beginning of") {
			continue
		}
		if fileWriter != nil {
			_, _ = fileWriter.Write([]byte(raw + "\n"))
		}
		entry := ParseLine(raw)
		if onEntry(entry) {
			count++
		}
	}

	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	if scanErr != nil {
		return count, scanErr
	}
	if ctx.Err() == nil && waitErr != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return count, fmt.Errorf("adb logcat process failed (%w): %s", waitErr, errText)
		}
		return count, fmt.Errorf("adb logcat process failed: %w", waitErr)
	}
	return count, nil
}
