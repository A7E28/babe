package main

import (
	"fmt"
	"strings"

	"github.com/A7E28/babe/internal/ui"
)

func printHelp() {
	fmt.Printf("%s %s\n\n", ui.BoldCyan("babe"), ui.Gray(version))
	fmt.Println(ui.White("Usage:"))
	fmt.Println("  babe [command] [flags]")
	fmt.Println("  babe [flags]")
	fmt.Println()
	fmt.Println(ui.White("Core Commands:"))
	fmt.Println("  (default)            Launch interactive terminal menu")
	fmt.Println("  setup                Install babe globally into user PATH")
	fmt.Println("  doctor               Run system, PATH conflict & USB diagnostics")
	fmt.Println("  tools                Manage Android SDK platform-tools (adb/fastboot)")
	fmt.Println("  update               Update platform-tools or babe executable")
	fmt.Println()
	fmt.Println(ui.White("Device & Fastboot Commands:"))
	fmt.Println("  device list          List attached ADB devices")
	fmt.Println("  device info          Display device hardware, OS, battery & root status")
	fmt.Println("  device reboot        Reboot device (system, bootloader, recovery, edl)")
	fmt.Println("  fastboot devices     List devices in Fastboot/Bootloader mode")
	fmt.Println("  fastboot info        Inspect bootloader lock, slot & variables")
	fmt.Println("  fastboot slot        Switch active A/B boot slot")
	fmt.Println("  fastboot flash       Flash partition image with SHA-256 verification")
	fmt.Println()
	fmt.Println(ui.White("Package, Image & Diagnostic Commands:"))
	fmt.Println("  apk inspect          Offline manifest, permissions, SDK & ABI analysis")
	fmt.Println("  apk install          Install standard APK or Split APK bundle")
	fmt.Println("  apk list             List installed packages on device")
	fmt.Println("  apk pull             Extract installed package APK from device")
	fmt.Println("  image inspect        Offline Android boot.img / init_boot.img header parser")
	fmt.Println("  logcat               Stream and filter device logcat messages")
	fmt.Println("  logcat crash         Dump device crash buffer and native tombstones")
	fmt.Println("  hash                 Compute MD5, SHA-1 & SHA-256 file hashes")
	fmt.Println()
	fmt.Println(ui.White("Flags:"))
	fmt.Println("  -h, --help           Show help for babe or a subcommand")
	fmt.Println("  -v, --version        Print babe version")
	fmt.Println()
	fmt.Printf("Use %s or %s for detailed flags and examples.\n",
		ui.Cyan("babe <command> --help"), ui.Cyan("babe help <command>"))
}

func handleHelpCommand(args []string) {
	if len(args) == 0 {
		printHelp()
		return
	}
	cmd := strings.ToLower(args[0])
	rest := args[1:]

	switch cmd {
	case "doctor":
		printDoctorHelp()
	case "tools":
		printToolsHelp()
	case "update":
		printUpdateHelp()
	case "setup":
		printSetupHelp()
	case "device", "devices":
		if len(rest) > 0 {
			switch strings.ToLower(rest[0]) {
			case "info":
				printDeviceInfoHelp()
				return
			case "reboot":
				printDeviceRebootHelp()
				return
			}
		}
		printDeviceHelp()
	case "info":
		printDeviceInfoHelp()
	case "reboot":
		printDeviceRebootHelp()
	case "fastboot", "fb":
		if len(rest) > 0 {
			switch strings.ToLower(rest[0]) {
			case "slot":
				printFastbootSlotHelp()
				return
			case "flash":
				printFastbootFlashHelp()
				return
			}
		}
		printFastbootHelp()
	case "apk":
		if len(rest) > 0 {
			switch strings.ToLower(rest[0]) {
			case "inspect", "info":
				printAPKInspectHelp()
				return
			case "install":
				printAPKInstallHelp()
				return
			case "list", "ls":
				printAPKListHelp()
				return
			case "pull":
				printAPKPullHelp()
				return
			}
		}
		printAPKHelp()
	case "image", "img":
		printImageHelp()
	case "logcat":
		if len(rest) > 0 && strings.EqualFold(rest[0], "crash") {
			printLogcatCrashHelp()
			return
		}
		printLogcatHelp()
	case "hash":
		printHashHelp()
	default:
		printHelp()
	}
}

func printDoctorHelp() {
	fmt.Println("Usage: babe doctor [flags]")
	fmt.Println("\nRun system readiness, PATH shadowing conflicts, and USB device diagnostics.")
	fmt.Println("\nFlags:")
	fmt.Println("  --fix-udev               Install and reload Linux Android USB udev rules")
	fmt.Println("  -h, --help               Show help for doctor")
	fmt.Println("\nExamples:")
	fmt.Println("  babe doctor")
	fmt.Println("  babe doctor --fix-udev")
}

func printToolsHelp() {
	fmt.Println("Usage: babe tools <subcommand>")
	fmt.Println("\nManage Android SDK platform-tools (adb and fastboot).")
	fmt.Println("\nSubcommands:")
	fmt.Println("  status, check            Check installation status and PATH availability")
	fmt.Println("  update, install          Download or update platform-tools to latest Google release")
	fmt.Println("  setup                    Configure babe binary and platform-tools in user PATH")
	fmt.Println("\nFlags:")
	fmt.Println("  -h, --help               Show help for tools")
	fmt.Println("\nExamples:")
	fmt.Println("  babe tools status")
	fmt.Println("  babe tools update")
}

func printUpdateHelp() {
	fmt.Println("Usage: babe update <tools|self> [flags]")
	fmt.Println("\nUpdate Android SDK platform-tools or the babe executable.")
	fmt.Println("\nTargets:")
	fmt.Println("  tools, platform-tools    Install or update Google Android platform-tools (adb/fastboot)")
	fmt.Println("  self, babe               Check and update babe executable to latest GitHub release")
	fmt.Println("\nFlags:")
	fmt.Println("  -f, --force              Force download/reinstall even if already up to date")
	fmt.Println("  -c, --check              Check for updates without downloading")
	fmt.Println("  -h, --help               Show help for update")
	fmt.Println("\nExamples:")
	fmt.Println("  babe update tools")
	fmt.Println("  babe update tools --force")
	fmt.Println("  babe update self")
	fmt.Println("  babe update self --check")
}

func printSetupHelp() {
	fmt.Println("Usage: babe setup [flags]")
	fmt.Println("\nInstall babe executable globally into user PATH.")
	fmt.Println("\nFlags:")
	fmt.Println("  -f, --force              Force overwrite canonical binary")
	fmt.Println("  -s, --status             Check setup status without modifying system")
	fmt.Println("  -h, --help               Show help for setup")
	fmt.Println("\nExamples:")
	fmt.Println("  babe setup")
	fmt.Println("  babe setup --status")
	fmt.Println("  babe setup --force")
}

func printHashHelp() {
	fmt.Println("Usage: babe hash <file-path>")
	fmt.Println("\nCompute and display MD5, SHA-1, and SHA-256 cryptographic hashes for a file.")
	fmt.Println("\nArguments:")
	fmt.Println("  <file-path>              Path to the file to hash")
	fmt.Println("\nFlags:")
	fmt.Println("  -h, --help               Show help for hash")
	fmt.Println("\nExamples:")
	fmt.Println("  babe hash boot.img")
	fmt.Println("  babe hash rom.zip")
}
