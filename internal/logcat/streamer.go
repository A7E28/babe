package logcat

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// configures the logcat streaming session.
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

// connects to `adb logcat`, streams entries in real-time, and terminates cleanly when ctx is cancelled.
func Stream(ctx context.Context, opts StreamOptions, handler LineHandler) error {
	adbBin := opts.ADBBinary
	if adbBin == "" {
		adbBin = "adb"
	}

	if opts.ClearBefore {
		clearArgs := []string{}
		if opts.Serial != "" {
			clearArgs = append(clearArgs, "-s", opts.Serial)
		}
		clearArgs = append(clearArgs, "logcat", "-c")
		_ = exec.CommandContext(ctx, adbBin, clearArgs...).Run()
	}

	args := []string{}
	if opts.Serial != "" {
		args = append(args, "-s", opts.Serial)
	}
	args = append(args, "logcat", "-b", "main,system", "-v", "time")

	if opts.MinLevel != LevelVerbose {
		args = append(args, fmt.Sprintf("*:%s", opts.MinLevel.String()))
	}

	var pids []string
	if opts.PIDFilter != "" {
		pids = append(pids, opts.PIDFilter)
	} else if opts.PackageName != "" {
		if isDigitsOnly(opts.PackageName) {
			pids = append(pids, opts.PackageName)
		} else {
			pidCmdArgs := []string{}
			if opts.Serial != "" {
				pidCmdArgs = append(pidCmdArgs, "-s", opts.Serial)
			}
			pidCmdArgs = append(pidCmdArgs, "shell", "pidof", opts.PackageName)
			out, err := exec.CommandContext(ctx, adbBin, pidCmdArgs...).Output()
			if err != nil {
				return fmt.Errorf("process %q is not running on device", opts.PackageName)
			}
			for _, p := range strings.Fields(string(out)) {
				if isDigitsOnly(p) {
					pids = append(pids, p)
				}
			}
			if len(pids) == 0 {
				return fmt.Errorf("process %q is not running on device", opts.PackageName)
			}
		}
	}

	for _, pid := range pids {
		args = append(args, "--pid="+pid)
	}

	cmd := exec.CommandContext(ctx, adbBin, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open stdout pipe for logcat: %w", err)
	}

	var fileWriter io.WriteCloser
	if opts.OutputFile != "" {
		f, err := os.Create(opts.OutputFile)
		if err != nil {
			return fmt.Errorf("create logcat output file: %w", err)
		}
		fileWriter = f
		defer fileWriter.Close()
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start logcat subprocess: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 512*1024)

	for scanner.Scan() {
		raw := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(raw), "--------- beginning of") {
			continue
		}
		if fileWriter != nil {
			_, _ = fileWriter.Write([]byte(raw + "\n"))
		}

		entry := ParseLine(raw)

		if opts.TagFilter != "" && !strings.Contains(strings.ToLower(entry.Tag), strings.ToLower(opts.TagFilter)) {
			continue
		}
		if entry.Level < opts.MinLevel {
			continue
		}

		if handler != nil {
			handler(entry)
		}
	}

	_ = cmd.Wait()
	return scanner.Err()
}

// harvests the device's crash ring buffer (`logcat -b crash -v time -d`) and exits immediately.
func DumpCrashLogs(ctx context.Context, adbBinary, serial, outputFile string, handler LineHandler) (int, error) {
	if adbBinary == "" {
		adbBinary = "adb"
	}

	args := []string{}
	if serial != "" {
		args = append(args, "-s", serial)
	}
	args = append(args, "logcat", "-b", "crash", "-v", "time", "-d")

	cmd := exec.CommandContext(ctx, adbBinary, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("open crash log pipe: %w", err)
	}

	var fileWriter io.WriteCloser
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return 0, fmt.Errorf("create crash output file: %w", err)
		}
		fileWriter = f
		defer fileWriter.Close()
	}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start crash log dump: %w", err)
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
		count++
		if handler != nil {
			handler(entry)
		}
	}

	_ = cmd.Wait()
	return count, scanner.Err()
}
