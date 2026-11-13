package main

import "fmt"

func printDeviceHelp() {
	fmt.Println("Usage: babe device <subcommand> [flags]")
	fmt.Println("\nManage and inspect connected ADB devices.")
	fmt.Println("\nSubcommands:")
	fmt.Println("  list                     List attached ADB devices and their state")
	fmt.Println("  info                     Display device hardware, OS, battery & root status")
	fmt.Println("  reboot [target]          Reboot device: system, bootloader, recovery, edl")
	fmt.Println("\nFlags:")
	fmt.Println("  -h, --help               Show help for device commands")
	fmt.Println("\nExamples:")
	fmt.Println("  babe device list")
	fmt.Println("  babe device info -s <serial>")
	fmt.Println("  babe device reboot bootloader")
	fmt.Println("  babe device reboot recovery -s <serial>")
}

func printDeviceInfoHelp() {
	fmt.Println("Usage: babe device info [-s <serial>]")
	fmt.Println("\nDisplay comprehensive device hardware, OS, battery, and root status.")
	fmt.Println("\nFlags:")
	fmt.Println("  -s, --serial <serial>    Target specific device serial (default: auto-detected)")
	fmt.Println("  -h, --help               Show help for device info")
	fmt.Println("\nExamples:")
	fmt.Println("  babe device info")
	fmt.Println("  babe device info -s 12345678")
}

func printDeviceRebootHelp() {
	fmt.Println("Usage: babe device reboot [mode] [-s <serial>]")
	fmt.Println("\nReboot connected device into specified target mode.")
	fmt.Println("\nModes:")
	fmt.Println("  system, normal           Reboot normally into Android system (default)")
	fmt.Println("  bootloader, fastboot     Reboot into Fastboot / Bootloader mode")
	fmt.Println("  recovery                 Reboot into Recovery mode")
	fmt.Println("  edl                      Reboot into Emergency Download Mode (Qualcomm EDL)")
	fmt.Println("\nFlags:")
	fmt.Println("  -s, --serial <serial>    Target specific device serial (default: auto-detected)")
	fmt.Println("  -h, --help               Show help for device reboot")
	fmt.Println("\nExamples:")
	fmt.Println("  babe device reboot")
	fmt.Println("  babe device reboot bootloader")
	fmt.Println("  babe device reboot recovery -s 12345678")
}

func printFastbootHelp() {
	fmt.Println("Usage: babe fastboot <subcommand> [args] [flags]")
	fmt.Println("\nManage devices in Fastboot / Bootloader mode.")
	fmt.Println("\nSubcommands:")
	fmt.Println("  devices                  List attached fastboot devices")
	fmt.Println("  info                     Inspect bootloader lock state, current slot & variables")
	fmt.Println("  slot <a|b>               Switch active A/B boot slot")
	fmt.Println("  flash <part> <image>     Flash partition image with SHA-256 verification")
	fmt.Println("\nFlags:")
	fmt.Println("  -s, --serial <serial>    Target specific fastboot device serial")
	fmt.Println("  -h, --help               Show help for fastboot commands")
	fmt.Println("\nExamples:")
	fmt.Println("  babe fastboot devices")
	fmt.Println("  babe fastboot info")
	fmt.Println("  babe fastboot slot b")
	fmt.Println("  babe fastboot flash boot boot.img")
}

func printFastbootSlotHelp() {
	fmt.Println("Usage: babe fastboot slot <a|b> [-s <serial>]")
	fmt.Println("\nSwitch active boot slot on A/B dual-partition devices.")
	fmt.Println("\nFlags:")
	fmt.Println("  -s, --serial <serial>    Target specific fastboot device serial")
	fmt.Println("  -h, --help               Show help for fastboot slot")
	fmt.Println("\nExamples:")
	fmt.Println("  babe fastboot slot a")
	fmt.Println("  babe fastboot slot b -s 12345678")
}

func printFastbootFlashHelp() {
	fmt.Println("Usage: babe fastboot flash <partition> <image-file> [-s <serial>]")
	fmt.Println("\nFlash a partition image with pre-flash SHA-256 integrity verification.")
	fmt.Println("\nArguments:")
	fmt.Println("  <partition>              Target partition name (e.g. boot, init_boot, vendor_boot, recovery)")
	fmt.Println("  <image-file>             Path to the image file to flash (.img)")
	fmt.Println("\nFlags:")
	fmt.Println("  -s, --serial <serial>    Target specific fastboot device serial")
	fmt.Println("  -h, --help               Show help for fastboot flash")
	fmt.Println("\nExamples:")
	fmt.Println("  babe fastboot flash boot boot.img")
	fmt.Println("  babe fastboot flash init_boot init_boot.img -s 12345678")
}

func printAPKHelp() {
	fmt.Println("Usage: babe apk <subcommand> [args] [flags]")
	fmt.Println("\nInspect, install, list, and extract Android packages.")
	fmt.Println("\nSubcommands:")
	fmt.Println("  inspect <file>           Offline manifest, SDK, ABI & certificate analysis")
	fmt.Println("  install <file>           Install standard APK or split APK bundle (.apks/.xapk/.apkm)")
	fmt.Println("  list                     List installed packages on device")
	fmt.Println("  pull <package>           Extract installed package APK from device")
	fmt.Println("\nFlags:")
	fmt.Println("  -h, --help               Show help for apk commands")
	fmt.Println("\nExamples:")
	fmt.Println("  babe apk inspect app.apk")
	fmt.Println("  babe apk inspect bundle.apks --all")
	fmt.Println("  babe apk install app.apk")
	fmt.Println("  babe apk list -3 -f google")
	fmt.Println("  babe apk pull com.example.app -o ./backup")
}

func printAPKInspectHelp() {
	fmt.Println("Usage: babe apk inspect <file.apk|.apks|.xapk|.apkm> [flags]")
	fmt.Println("\nInspect package manifest, SDK versions, native ABIs, and signing certificates offline.")
	fmt.Println("\nFlags:")
	fmt.Println("  -a, --all                Display all declared permissions (including normal permissions)")
	fmt.Println("  -h, --help               Show help for apk inspect")
	fmt.Println("\nExamples:")
	fmt.Println("  babe apk inspect app.apk")
	fmt.Println("  babe apk inspect bundle.apks --all")
}

func printAPKInstallHelp() {
	fmt.Println("Usage: babe apk install <file.apk|.apks|.xapk|.apkm> [flags]")
	fmt.Println("\nInstall a standard APK or split APK bundle onto a connected device.")
	fmt.Println("\nFlags:")
	fmt.Println("  -s, --serial <serial>    Target specific device serial (default: auto-detected)")
	fmt.Println("  -h, --help               Show help for apk install")
	fmt.Println("\nExamples:")
	fmt.Println("  babe apk install app.apk")
	fmt.Println("  babe apk install bundle.apks -s 12345678")
}

func printAPKListHelp() {
	fmt.Println("Usage: babe apk list [flags]")
	fmt.Println("\nList installed packages on connected device.")
	fmt.Println("\nFlags:")
	fmt.Println("  -3                       List third-party (user-installed) packages only")
	fmt.Println("  -f, --filter <string>    Filter package names matching substring")
	fmt.Println("  -s, --serial <serial>    Target specific device serial (default: auto-detected)")
	fmt.Println("  -h, --help               Show help for apk list")
	fmt.Println("\nExamples:")
	fmt.Println("  babe apk list")
	fmt.Println("  babe apk list -3")
	fmt.Println("  babe apk list -3 -f camera")
}

func printAPKPullHelp() {
	fmt.Println("Usage: babe apk pull <package-name> [flags]")
	fmt.Println("\nExtract and pull installed APK files from device to host.")
	fmt.Println("\nFlags:")
	fmt.Println("  -o, --output <dir>       Destination directory (default: current directory)")
	fmt.Println("  -s, --serial <serial>    Target specific device serial (default: auto-detected)")
	fmt.Println("  -h, --help               Show help for apk pull")
	fmt.Println("\nExamples:")
	fmt.Println("  babe apk pull com.example.app")
	fmt.Println("  babe apk pull com.google.android.youtube -o ./backup")
}

func printLogcatHelp() {
	fmt.Println("Usage: babe logcat [flags]")
	fmt.Println("\nStream and filter device logcat messages in real time.")
	fmt.Println("\nSubcommands:")
	fmt.Println("  crash                    Dump device crash buffer and native tombstones")
	fmt.Println("\nFlags:")
	fmt.Println("  -v, --level <level>      Minimum log level: V, D, I, W, E, F (default: V)")
	fmt.Println("  -t, --tag <tag>          Filter log entries matching tag substring")
	fmt.Println("  -p, --package <pkg|pid>  Filter by package name or process ID")
	fmt.Println("  -s, --serial <serial>    Target specific device serial (default: auto-detected)")
	fmt.Println("  -o, --output <file>      Tee log output to a file")
	fmt.Println("  -h, --help               Show help for logcat")
	fmt.Println("\nExamples:")
	fmt.Println("  babe logcat -v W")
	fmt.Println("  babe logcat -p com.example.app")
	fmt.Println("  babe logcat -t ActivityManager -v I")
	fmt.Println("  babe logcat -s 12345678 -o log.txt")
}

func printLogcatCrashHelp() {
	fmt.Println("Usage: babe logcat crash [flags]")
	fmt.Println("\nDump device crash logs, tombstones, and native crash buffers.")
	fmt.Println("\nFlags:")
	fmt.Println("  -s, --serial <serial>    Target specific device serial (default: auto-detected)")
	fmt.Println("  -o, --output <file>      Save crash report to specified file")
	fmt.Println("  -h, --help               Show help for logcat crash")
	fmt.Println("\nExamples:")
	fmt.Println("  babe logcat crash")
	fmt.Println("  babe logcat crash -o crash_report.txt")
	fmt.Println("  babe logcat crash -s 12345678 -o crash.log")
}

func printImageHelp() {
	fmt.Println("Usage: babe image inspect <boot.img|init_boot.img>")
	fmt.Println("\nOffline parser for Android boot.img and init_boot.img headers (v0 through v4).")
	fmt.Println("\nArguments:")
	fmt.Println("  <file>                   Path to boot.img or init_boot.img file")
	fmt.Println("\nFlags:")
	fmt.Println("  -h, --help               Show help for image inspect")
	fmt.Println("\nExamples:")
	fmt.Println("  babe image inspect boot.img")
	fmt.Println("  babe image inspect /path/to/init_boot.img")
}
