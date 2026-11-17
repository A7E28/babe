package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/A7E28/babe/internal/device"
	"github.com/A7E28/babe/internal/fastboot"
	"github.com/A7E28/babe/internal/platform"
)

func runCLIWithBuffers(args ...string) (string, string, error) {
	cmd := newRootCmd(device.NewClient(), fastboot.NewClient(), platform.DefaultManager())
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := executeRootCmd(cmd, args)
	return stdout.String(), stderr.String(), err
}

func TestCLI_VersionJSON(t *testing.T) {
	out, errOut, err := runCLIWithBuffers("version", "--json")
	if err != nil {
		t.Fatalf("version --json failed: %v (stderr: %s)", err, errOut)
	}
	if errOut != "" {
		t.Errorf("expected empty stderr, got %q", errOut)
	}

	var got versionReport
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON output %q: %v", out, err)
	}
	if got.Version != version {
		t.Errorf("got version %q, want %q", got.Version, version)
	}
}

func TestCLI_HashJSON_SuccessAndError(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "sample.bin")
	if err := os.WriteFile(tmpFile, []byte("babe-json-test"), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	out, errOut, err := runCLIWithBuffers("hash", tmpFile, "--json")
	if err != nil {
		t.Fatalf("hash --json failed: %v (stderr: %s)", err, errOut)
	}
	var got hashReport
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON output %q: %v", out, err)
	}
	if got.File != tmpFile || len(got.MD5) != 32 || len(got.SHA1) != 40 || len(got.SHA256) != 64 {
		t.Errorf("unexpected hash report: %+v", got)
	}

	missing := filepath.Join(t.TempDir(), "does-not-exist.bin")
	out, errOut, err = runCLIWithBuffers("hash", missing, "--json")
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
	if out != "" {
		t.Errorf("expected empty stdout on error, got %q", out)
	}
	var errGot errorReport
	if err := json.Unmarshal([]byte(errOut), &errGot); err != nil {
		t.Fatalf("expected JSON error on stderr, got %q: %v", errOut, err)
	}
	if errGot.Error == "" {
		t.Errorf("expected non-empty error field")
	}
}

func TestCLI_ImageInspectJSON(t *testing.T) {
	buf := make([]byte, 2048)
	copy(buf[0:8], []byte("ANDROID!"))
	binary.LittleEndian.PutUint32(buf[8:12], 123456)
	binary.LittleEndian.PutUint32(buf[12:16], 654321)
	binary.LittleEndian.PutUint32(buf[40:44], 4)
	copy(buf[44:], []byte("console=ttyMSM0\x00"))

	imgPath := filepath.Join(t.TempDir(), "boot.img")
	if err := os.WriteFile(imgPath, buf, 0644); err != nil {
		t.Fatalf("write boot.img: %v", err)
	}

	for _, args := range [][]string{{"image", "inspect", imgPath, "--json"}, {"image", imgPath, "--json"}} {
		out, errOut, err := runCLIWithBuffers(args...)
		if err != nil {
			t.Fatalf("%v failed: %v (stderr: %s)", args, err, errOut)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(out), &decoded); err != nil {
			t.Fatalf("invalid JSON for %v: %v\n%s", args, err, out)
		}
		if decoded["header_version"] != float64(4) || decoded["kernel_size"] != float64(123456) {
			t.Errorf("unexpected boot image JSON for %v: %v", args, decoded)
		}
	}
}

func TestCLI_APKInspectJSON(t *testing.T) {
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, _ := zw.Create("AndroidManifest.xml")
	_, _ = w.Write([]byte("not-axml"))
	wDex, _ := zw.Create("classes.dex")
	_, _ = wDex.Write([]byte("dex"))
	_ = zw.Close()

	apkPath := filepath.Join(t.TempDir(), "sample.apk")
	if err := os.WriteFile(apkPath, zipBuf.Bytes(), 0644); err != nil {
		t.Fatalf("write sample apk: %v", err)
	}

	out, errOut, err := runCLIWithBuffers("apk", "inspect", apkPath, "--json")
	if err != nil {
		t.Fatalf("apk inspect --json failed: %v (stderr: %s)", err, errOut)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if decoded["dex_count"] != float64(1) {
		t.Errorf("expected dex_count=1, got %v", decoded["dex_count"])
	}
	if _, ok := decoded["abis"].([]any); !ok {
		t.Errorf("expected abis to serialize as JSON array, got %T", decoded["abis"])
	}
	if _, ok := decoded["permissions"].([]any); !ok {
		t.Errorf("expected permissions to serialize as JSON array, got %T", decoded["permissions"])
	}
}

func TestCLI_SetupAndDoctorAndToolsStatusJSON(t *testing.T) {
	for _, args := range [][]string{
		{"setup", "--status", "--json"},
		{"tools", "status", "--json"},
		{"doctor", "--json"},
	} {
		out, errOut, err := runCLIWithBuffers(args...)
		if err != nil {
			t.Fatalf("%v failed: %v (stderr: %s)", args, err, errOut)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(out), &decoded); err != nil {
			t.Fatalf("invalid JSON for %v: %v\n%s", args, err, out)
		}
		if len(decoded) == 0 {
			t.Errorf("expected non-empty JSON object for %v", args)
		}
	}
}

func TestCLI_LogcatInvalidLevelJSONError(t *testing.T) {
	out, errOut, err := runCLIWithBuffers("logcat", "-v", "NOPE", "--json")
	if err == nil {
		t.Fatalf("expected error for invalid log level")
	}
	if out != "" {
		t.Errorf("expected empty stdout on error, got %q", out)
	}
	var errGot errorReport
	if err := json.Unmarshal([]byte(errOut), &errGot); err != nil {
		t.Fatalf("expected JSON error on stderr, got %q: %v", errOut, err)
	}
	if !strings.Contains(errGot.Error, "NOPE") {
		t.Errorf("expected error message to mention NOPE, got %q", errGot.Error)
	}
}

func TestCLI_CobraParseAndArgErrorsJSON(t *testing.T) {
	cases := [][]string{
		{"hash", "--json"},
		{"version", "--unknown-flag", "--json"},
		{"no-such-command", "--json"},
	}

	for _, args := range cases {
		out, errOut, err := runCLIWithBuffers(args...)
		if err == nil {
			t.Fatalf("expected error for %v", args)
		}
		if out != "" {
			t.Errorf("expected empty stdout on error for %v, got %q", args, out)
		}
		var errGot errorReport
		if jErr := json.Unmarshal([]byte(errOut), &errGot); jErr != nil {
			t.Fatalf("expected JSON error on stderr for %v, got %q: %v", args, errOut, jErr)
		}
		if errGot.Error == "" {
			t.Errorf("expected non-empty error field for %v", args)
		}
	}
}

func TestHasJSONFlag(t *testing.T) {
	if !hasJSONFlag([]string{"doctor", "--json"}) {
		t.Errorf("expected --json to be detected")
	}
	if !hasJSONFlag([]string{"--json=true", "version"}) {
		t.Errorf("expected --json=true to be detected")
	}
	if hasJSONFlag([]string{"hash", "--", "--json"}) {
		t.Errorf("expected --json after '--' terminator to be ignored")
	}
}
