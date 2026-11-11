package fastboot

import (
	"reflect"
	"testing"
)

func TestParseDevicesOutput(t *testing.T) {
	raw := "RF8M123456X\tfastboot\n9900112233\tfastbootd\n"
	got := ParseDevicesOutput(raw)
	want := []Device{
		{Serial: "RF8M123456X", Mode: "fastboot"},
		{Serial: "9900112233", Mode: "fastbootd"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseDevicesOutput() = %+v, want %+v", got, want)
	}
}

func TestParseGetvarAll(t *testing.T) {
	raw := `(bootloader) product: husky
(bootloader) version-bootloader: ripcurrent-14.4-11429210
(bootloader) version-baseband: g5300q-231218-240202
(bootloader) serialno: 38041FDJH0012X
(bootloader) secure: yes
(bootloader) unlocked: yes
(bootloader) current-slot: b
(bootloader) slot-count: 2
(bootloader) max-download-size: 0x10000000
all: Done!!
Finished. Total time: 0.025s`

	info := ParseGetvarAll(raw, "38041FDJH0012X")
	if info.Product != "husky" {
		t.Errorf("got Product %q, want husky", info.Product)
	}
	if info.Unlocked != "yes" {
		t.Errorf("got Unlocked %q, want yes", info.Unlocked)
	}
	if info.CurrentSlot != "b" {
		t.Errorf("got CurrentSlot %q, want b", info.CurrentSlot)
	}
	if info.MaxDownloadSize != "0x10000000" {
		t.Errorf("got MaxDownloadSize %q, want 0x10000000", info.MaxDownloadSize)
	}
}
