package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type Target struct {
	ID   string
	OS   string
	Arch string
	Ext  string
}

// Tier-1 supported release targets
var targets = []Target{
	{ID: "win64", OS: "windows", Arch: "amd64", Ext: ".exe"},
	{ID: "win-arm64", OS: "windows", Arch: "arm64", Ext: ".exe"},
	{ID: "linux64", OS: "linux", Arch: "amd64", Ext: ""},
	{ID: "linux-arm64", OS: "linux", Arch: "arm64", Ext: ""},
}

// Auxiliary targets (legacy)
var legacyTargets = []Target{
	{ID: "win32", OS: "windows", Arch: "386", Ext: ".exe"},
}

func main() {
	arg := "current"
	if len(os.Args) > 1 {
		arg = strings.ToLower(strings.TrimSpace(os.Args[1]))
	}

	version := resolveVersion()

	switch arg {
	case "help", "-h", "--help":
		printHelp()
	case "clean":
		cleanDist()
	case "test":
		runTests()
	case "all":
		buildTargets(targets, version, false)
	case "release":
		cleanDist()
		buildTargets(targets, version, true)
		generateChecksums()
	case "windows", "win":
		buildFilter("windows", version, false)
	case "linux":
		buildFilter("linux", version, false)
	default:
		if t := findTarget(arg); t != nil {
			buildSingle(*t, version, false)
			return
		}
		if arg == "current" {
			buildCurrent(version)
			return
		}
		fmt.Printf("[ERROR] Unknown build target: %s\n\n", arg)
		printHelp()
		os.Exit(1)
	}
}

func resolveVersion() string {
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	out, err := cmd.Output()
	if err == nil {
		ver := strings.TrimSpace(string(out))
		if ver != "" {
			return ver
		}
	}
	return "dev"
}

func buildCurrent(version string) {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	outName := fmt.Sprintf("babe%s", ext)
	outPath := filepath.Join("dist", outName)

	t := Target{
		ID:   "current",
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
		Ext:  ext,
	}

	_ = os.MkdirAll("dist", 0755)
	executeBuild(t, outPath, version, false)
}

func buildFilter(osFilter, version string, release bool) {
	var filtered []Target
	for _, t := range targets {
		if t.OS == osFilter {
			filtered = append(filtered, t)
		}
	}
	buildTargets(filtered, version, release)
}

func buildTargets(list []Target, version string, release bool) {
	_ = os.MkdirAll("dist", 0755)
	fmt.Printf("Building %d target(s) [Version: %s | Release: %v]\n", len(list), version, release)
	fmt.Println("--------------------------------------------------")

	start := time.Now()
	for _, t := range list {
		outName := fmt.Sprintf("babe-%s-%s%s", t.OS, t.Arch, t.Ext)
		if release {
			outName = fmt.Sprintf("babe-%s-%s-%s%s", version, t.OS, t.Arch, t.Ext)
		}
		outPath := filepath.Join("dist", outName)
		if err := executeBuild(t, outPath, version, release); err != nil {
			fmt.Printf("[ERROR] %s failed: %v\n", t.ID, err)
			os.Exit(1)
		}
	}

	fmt.Printf("\n[OK] All targets built in %s\n", time.Since(start).Round(time.Millisecond))
}

func buildSingle(t Target, version string, release bool) {
	_ = os.MkdirAll("dist", 0755)
	outName := fmt.Sprintf("babe-%s-%s%s", t.OS, t.Arch, t.Ext)
	outPath := filepath.Join("dist", outName)
	if err := executeBuild(t, outPath, version, release); err != nil {
		os.Exit(1)
	}
}

func executeBuild(t Target, outPath, version string, release bool) error {
	ldflags := fmt.Sprintf("-X main.version=%s", version)
	if release {
		ldflags += " -s -w"
	}

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", outPath, "./cmd/babe")
	cmd.Env = append(os.Environ(),
		"GOOS="+t.OS,
		"GOARCH="+t.Arch,
		"CGO_ENABLED=0",
	)

	start := time.Now()
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("[FAIL] %-12s -> %s\n%s\n", t.ID, outPath, string(out))
		return err
	}

	fi, _ := os.Stat(outPath)
	sizeMB := float64(fi.Size()) / (1024 * 1024)
	fmt.Printf("[OK]   %-12s -> %-28s [%.2f MB, %s]\n", t.ID, outPath, sizeMB, time.Since(start).Round(time.Millisecond))
	return nil
}

func generateChecksums() {
	files, err := os.ReadDir("dist")
	if err != nil {
		return
	}

	checksumFile := filepath.Join("dist", "checksums.txt")
	out, err := os.Create(checksumFile)
	if err != nil {
		return
	}
	defer out.Close()

	fmt.Println("\nGenerating SHA256 checksums:")
	for _, f := range files {
		if f.IsDir() || f.Name() == "checksums.txt" {
			continue
		}
		path := filepath.Join("dist", f.Name())
		hash := calcSHA256(path)
		line := fmt.Sprintf("%s  %s\n", hash, f.Name())
		_, _ = out.WriteString(line)
		fmt.Printf("  %s  %s\n", hash[:16]+"...", f.Name())
	}
	fmt.Printf("[OK] Saved checksums to %s\n", checksumFile)
}

func calcSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	_, _ = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}

func runTests() {
	cmd := exec.Command("go", "test", "-v", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}

func cleanDist() {
	_ = os.RemoveAll("dist")
	fmt.Println("[OK] Cleaned dist/ directory.")
}

func findTarget(id string) *Target {
	for _, t := range append(targets, legacyTargets...) {
		if t.ID == id || fmt.Sprintf("%s-%s", t.OS, t.Arch) == id {
			return &t
		}
	}
	return nil
}

func printHelp() {
	fmt.Println("babe Build System (Go)")
	fmt.Println("Usage: go run build.go [target]")
	fmt.Println("\nTargets: (no arg), win64, win-arm64, win32, linux64, linux-arm64, windows, linux, all, release, test, clean")
}
