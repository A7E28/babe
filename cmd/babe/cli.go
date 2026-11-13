package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/logcat"
	"github.com/A7E28/babe/internal/platform"
	"github.com/A7E28/babe/internal/ui"
)

func handleCLI(args []string) {
	cmd := strings.ToLower(args[0])
	rest := args[1:]
	adbClient := device.NewClient()
	fbClient := fastboot.NewClient()
	pm := platform.DefaultManager()

	switch cmd {
	case "version", "-v", "--version":
		fmt.Printf("babe %s\n", version)
	case "doctor":
		if len(rest) > 0 && isHelpArg(rest[0]) {
			printDoctorHelp()
			return
		}
		fs := flag.NewFlagSet("doctor", flag.ExitOnError)
		fs.Usage = printDoctorHelp
		fixUdev := fs.Bool("fix-udev", false, "Install/reload Linux Android USB udev rules")
		_ = fs.Parse(reorderFlags(rest, map[string]bool{"--fix-udev": true, "-fix-udev": true}))
		handleDoctorCLI(adbClient, fbClient, pm, *fixUdev)
	case "tools":
		handleToolsCLI(adbClient, pm, rest)
	case "update":
		handleUpdateCLI(adbClient, pm, rest)
	case "setup":
		handleSetupCLI(pm, rest)
	case "device", "devices":
		if cmd == "devices" && len(rest) == 0 {
			handleDeviceListCLI(adbClient)
			return
		}
		handleDeviceDomainCLI(adbClient, rest)
	case "info":
		handleDeviceDomainCLI(adbClient, append([]string{"info"}, rest...))
	case "reboot":
		handleDeviceDomainCLI(adbClient, append([]string{"reboot"}, rest...))
	case "fastboot", "fb":
		handleFastbootDomainCLI(fbClient, rest)
	case "apk":
		handleAPKDomainCLI(adbClient, rest)
	case "install":
		if len(rest) == 0 || strings.EqualFold(rest[0], "self") || strings.EqualFold(rest[0], "babe") || strings.HasPrefix(rest[0], "-") {
			handleSetupCLI(pm, rest)
			return
		}
		handleAPKDomainCLI(adbClient, append([]string{"install"}, rest...))
	case "packages", "pkgs":
		handleAPKDomainCLI(adbClient, append([]string{"list"}, rest...))
	case "pull-apk":
		handleAPKDomainCLI(adbClient, append([]string{"pull"}, rest...))
	case "image", "img":
		handleImageDomainCLI(rest)
	case "logcat":
		handleLogcatDomainCLI(adbClient, rest)
	case "hash":
		if len(rest) < 1 {
			printHashHelp()
			os.Exit(1)
		}
		if isHelpArg(rest[0]) {
			printHashHelp()
			return
		}
		printFileHashes(rest[0])
	case "udev":
		handleUdevCLI()
	case "help", "-h", "--help":
		if len(rest) > 0 {
			handleHelpCommand(rest)
			return
		}
		printHelp()
	default:
		ui.Error("Unknown command %q. Run 'babe help' for usage.", cmd)
		os.Exit(1)
	}
}

func handleLogcatDomainCLI(client *device.Client, args []string) {
	if len(args) > 0 && isHelpArg(args[0]) {
		printLogcatHelp()
		return
	}
	if len(args) > 0 && strings.EqualFold(args[0], "crash") {
		if len(args) > 1 && isHelpArg(args[1]) {
			printLogcatCrashHelp()
			return
		}
		fs := flag.NewFlagSet("logcat crash", flag.ExitOnError)
		fs.Usage = printLogcatCrashHelp
		serial := fs.String("s", "", "Device serial")
		fs.StringVar(serial, "serial", "", "Device serial")
		out := fs.String("o", "", "Output file path")
		fs.StringVar(out, "output", "", "Output file path")
		_ = fs.Parse(reorderFlags(args[1:], nil))
		handleCrashLogCLI(client, *serial, *out)
		return
	}

	fs := flag.NewFlagSet("logcat", flag.ExitOnError)
	fs.Usage = printLogcatHelp
	serial := fs.String("s", "", "Device serial")
	fs.StringVar(serial, "serial", "", "Device serial")
	level := fs.String("v", "V", "Log level")
	fs.StringVar(level, "level", "V", "Log level")
	tag := fs.String("t", "", "Tag filter")
	fs.StringVar(tag, "tag", "", "Tag filter")
	pkg := fs.String("p", "", "Package or PID filter")
	fs.StringVar(pkg, "package", "", "Package or PID filter")
	out := fs.String("o", "", "Output file path")
	fs.StringVar(out, "output", "", "Output file path")
	_ = fs.Parse(reorderFlags(args, nil))

	if fs.NArg() > 0 {
		ui.Error("Unexpected logcat argument %q. Run 'babe logcat --help' for usage.", fs.Arg(0))
		os.Exit(1)
	}
	parsedLevel, err := logcat.ValidateLevel(*level)
	if err != nil {
		ui.Error("%v. Valid levels: V, D, I, W, E, F.", err)
		os.Exit(1)
	}
	handleLogcatCLI(client, logcat.StreamOptions{
		Serial: *serial, MinLevel: parsedLevel, TagFilter: *tag, PackageName: *pkg, OutputFile: *out,
	})
}

func handleLogcatCLI(client *device.Client, opts logcat.StreamOptions) {
	targetSerial, err := resolveTargetDevice(client, opts.Serial)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
	opts.Serial = targetSerial
	opts.ADBBinary = client.BinaryPath()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err = logcat.Stream(ctx, opts, func(entry logcat.Entry) {
		fmt.Println(logcat.Colorize(entry))
	})
	if err != nil && ctx.Err() == nil {
		ui.Error("Logcat stream error: %v", err)
		os.Exit(1)
	}
}

func handleCrashLogCLI(client *device.Client, serial, outFile string) {
	targetSerial, err := resolveTargetDevice(client, serial)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	count, err := logcat.DumpCrashLogs(context.Background(), client.BinaryPath(), targetSerial, outFile, func(entry logcat.Entry) {
		fmt.Println(logcat.Colorize(entry))
	})
	if err != nil {
		ui.Error("Crash log dump failed: %v", err)
		os.Exit(1)
	}
	if count == 0 {
		ui.Info("No crash entries found in device crash buffer.")
		return
	}
	ui.Info("Dumped %d crash log line(s) from device %s.", count, targetSerial)
	if outFile != "" {
		ui.Success("Saved crash report to %s", outFile)
	}
}
