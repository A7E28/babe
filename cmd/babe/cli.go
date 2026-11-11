package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

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
		fs := flag.NewFlagSet("doctor", flag.ExitOnError)
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
			fmt.Println("Usage: babe hash <file-path>")
			os.Exit(1)
		}
		printFileHashes(rest[0])
	case "udev":
		handleUdevCLI()
	case "help", "-h", "--help":
		printHelp()
	default:
		ui.Error("Unknown command %q. Run 'babe help' for usage.", cmd)
		os.Exit(1)
	}
}

func handleLogcatDomainCLI(client *device.Client, args []string) {
	if len(args) > 0 && strings.EqualFold(args[0], "crash") {
		fs := flag.NewFlagSet("logcat crash", flag.ExitOnError)
		serial := fs.String("s", "", "Device serial")
		fs.StringVar(serial, "serial", "", "Device serial")
		out := fs.String("o", "", "Output file path")
		fs.StringVar(out, "output", "", "Output file path")
		_ = fs.Parse(reorderFlags(args[1:], nil))
		handleCrashLogCLI(client, *serial, *out)
		return
	}

	fs := flag.NewFlagSet("logcat", flag.ExitOnError)
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
		ui.Error("Unexpected logcat argument %q. Run 'babe help' for usage.", fs.Arg(0))
		os.Exit(1)
	}

	parsedLevel, err := logcat.ValidateLevel(*level)
	if err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	handleLogcatCLI(client, logcat.StreamOptions{
		Serial:      *serial,
		MinLevel:    parsedLevel,
		TagFilter:   *tag,
		PackageName: *pkg,
		OutputFile:  *out,
	})
}

func printHelp() {
	fmt.Printf("%s %s\n\n", ui.BoldCyan("babe"), ui.Gray(version))
	fmt.Println(ui.White("Usage:"))
	fmt.Println("  babe                                      Launch interactive terminal menu")
	fmt.Println("  babe setup [-f|--force] [-s|--status]     Install babe globally into user PATH")
	fmt.Println("  babe doctor [--fix-udev]                  Run system, PATH conflict & USB diagnostics")
	fmt.Println("  babe tools <status|update>                Manage Android SDK platform-tools (adb/fastboot)")
	fmt.Println("  babe update <tools|self> [-f] [-c]        Update platform-tools or babe executable")
	fmt.Println()
	fmt.Println(ui.White("Device & Fastboot Commands:"))
	fmt.Println("  babe device list                          List attached ADB devices")
	fmt.Println("  babe device info [-s <serial>]            Display hardware, OS, battery & root telemetry")
	fmt.Println("  babe device reboot [mode] [-s <serial>]   Reboot: system, bootloader, recovery, edl")
	fmt.Println("  babe fastboot devices                     List devices in Fastboot/Bootloader mode")
	fmt.Println("  babe fastboot info [-s <serial>]          Inspect bootloader lock, slot & variables")
	fmt.Println("  babe fastboot slot <a|b> [-s <serial>]    Switch active A/B boot slot")
	fmt.Println("  babe fastboot flash <part> <img> [-s ser] Hash-verified partition flash")
	fmt.Println()
	fmt.Println(ui.White("APK, Firmware Image & Forensics Commands:"))
	fmt.Println("  babe apk inspect <file> [-a|--all]        Offline manifest, SDK, ABI & cert analysis")
	fmt.Println("  babe apk install <file.apk|.apks> [-s s]  Install standard APK or Split APK bundle")
	fmt.Println("  babe apk list [-3] [-f <str>] [-s <ser>]  List installed packages on device")
	fmt.Println("  babe apk pull <pkg> [-o <dir>] [-s <ser>] Extract installed package APK from device")
	fmt.Println("  babe image inspect <boot.img>             Offline boot.img / init_boot.img header parser")
	fmt.Println("  babe logcat [flags]                       Stream real-time colored logcat")
	fmt.Println("  babe logcat crash [-o <file>] [-s <ser>]  Harvest native tombstone & crash buffer")
	fmt.Println("  babe hash <file-path>                     Single-pass MD5, SHA1 & SHA256 file hashes")
}
