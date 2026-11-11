package device

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseDevicesOutput(t *testing.T) {
	sampleOutput := `List of devices attached
emulator-5554          device product:sdk_gphone64_arm64 model:sdk_gphone64_arm64 device:emu64a transport_id:1
RF8M123456X            unauthorized usb:1-1 transport_id:2
192.168.1.50:5555      device product:husky model:Pixel_7_Pro device:husky transport_id:3
`

	devices := ParseDevicesOutput(sampleOutput)
	if len(devices) != 3 {
		t.Fatalf("expected 3 devices, got %d", len(devices))
	}

	expected := []Device{
		{
			Serial: "emulator-5554",
			State:  StateDevice,
			Model:  "sdk_gphone64_arm64",
		},
		{
			Serial: "RF8M123456X",
			State:  StateUnauthorized,
			Model:  "",
		},
		{
			Serial: "192.168.1.50:5555",
			State:  StateDevice,
			Model:  "Pixel_7_Pro",
		},
	}

	if !reflect.DeepEqual(devices, expected) {
		t.Errorf("got %+v, want %+v", devices, expected)
	}
}

func TestParseGetprop(t *testing.T) {
	sampleOutput := `[ro.build.version.release]: [14]
[ro.build.version.sdk]: [34]
[ro.product.brand]: [google]
[ro.product.model]: [Pixel 7]
[ro.product.cpu.abi]: [arm64-v8a]
`

	props := parseGetprop(sampleOutput)

	if props["ro.product.model"] != "Pixel 7" {
		t.Errorf("model mismatch: got %s", props["ro.product.model"])
	}
	if props["ro.build.version.sdk"] != "34" {
		t.Errorf("sdk mismatch: got %s", props["ro.build.version.sdk"])
	}
	if props["ro.product.cpu.abi"] != "arm64-v8a" {
		t.Errorf("abi mismatch: got %s", props["ro.product.cpu.abi"])
	}
}

func TestParseADBVersionOutput(t *testing.T) {
	output := `Android Debug Bridge version 1.0.41
Version 37.0.1-15733141
Installed as C:\Users\User\platform-tools\adb.exe
Running on Windows 10.0.26300`

	ver, binPath := ParseADBVersionOutput(output)
	if ver != "37.0.1" {
		t.Errorf("expected version 37.0.1, got %q", ver)
	}
	if binPath != `C:\Users\User\platform-tools\adb.exe` {
		t.Errorf("expected binary path C:\\Users\\User\\platform-tools\\adb.exe, got %q", binPath)
	}
}

func TestValidateDeviceState(t *testing.T) {
	if err := ValidateDeviceState(Device{Serial: "S1", State: StateDevice}); err != nil {
		t.Errorf("expected nil error for StateDevice, got %v", err)
	}
	if err := ValidateDeviceState(Device{Serial: "S2", State: StateUnauthorized}); !errors.Is(err, ErrDeviceUnauthorized) {
		t.Errorf("expected ErrDeviceUnauthorized for StateUnauthorized, got %v", err)
	}
	if err := ValidateDeviceState(Device{Serial: "S3", State: StateOffline}); !errors.Is(err, ErrDeviceOffline) {
		t.Errorf("expected ErrDeviceOffline for StateOffline, got %v", err)
	}
	if err := ValidateDeviceState(Device{Serial: "S4", State: StateRecovery}); !errors.Is(err, ErrDeviceNotReady) {
		t.Errorf("expected ErrDeviceNotReady for StateRecovery, got %v", err)
	}
}
