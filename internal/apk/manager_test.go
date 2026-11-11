package apk

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParsePackageList(t *testing.T) {
	raw := "package:com.android.settings\r\npackage:org.mozilla.firefox\n\npackage:com.google.android.gms\n"
	got := ParsePackageList(raw)
	want := []string{
		"com.android.settings",
		"org.mozilla.firefox",
		"com.google.android.gms",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParsePackageList() = %v, want %v", got, want)
	}
}

func TestParsePackagePath(t *testing.T) {
	raw := "package:/data/app/~~abc123==/org.mozilla.firefox-xyz==/base.apk\npackage:/data/app/~~abc123==/org.mozilla.firefox-xyz==/split_config.arm64_v8a.apk\n"
	got := ParsePackagePath(raw)
	want := "/data/app/~~abc123==/org.mozilla.firefox-xyz==/base.apk"

	if got != want {
		t.Errorf("ParsePackagePath() = %q, want %q", got, want)
	}
}

func buildSyntheticAXML() []byte {
	// Strings: [0]="manifest", [1]="package", [2]="com.example.demo", [3]="versionCode", [4]="versionName", [5]="1.2.3"
	strs := []string{"manifest", "package", "com.example.demo", "versionCode", "versionName", "1.2.3"}
	var strData bytes.Buffer
	offsets := make([]uint32, len(strs))
	for i, s := range strs {
		offsets[i] = uint32(strData.Len())
		strData.WriteByte(byte(len(s)))
		strData.WriteByte(byte(len(s)))
		strData.WriteString(s)
		strData.WriteByte(0)
	}
	for strData.Len()%4 != 0 {
		strData.WriteByte(0)
	}

	poolHeaderSize := 28
	offsetsSize := len(strs) * 4
	stringsStart := uint32(poolHeaderSize + offsetsSize)
	poolChunkSize := uint32(int(stringsStart) + strData.Len())

	var poolChunk bytes.Buffer
	_ = binary.Write(&poolChunk, binary.LittleEndian, uint16(0x0001))
	_ = binary.Write(&poolChunk, binary.LittleEndian, uint16(poolHeaderSize))
	_ = binary.Write(&poolChunk, binary.LittleEndian, poolChunkSize)
	_ = binary.Write(&poolChunk, binary.LittleEndian, uint32(len(strs)))
	_ = binary.Write(&poolChunk, binary.LittleEndian, uint32(0))
	_ = binary.Write(&poolChunk, binary.LittleEndian, uint32(1<<8)) // UTF-8 flag
	_ = binary.Write(&poolChunk, binary.LittleEndian, stringsStart)
	_ = binary.Write(&poolChunk, binary.LittleEndian, uint32(0))
	for _, off := range offsets {
		_ = binary.Write(&poolChunk, binary.LittleEndian, off)
	}
	poolChunk.Write(strData.Bytes())

	// Start Element chunk for <manifest package="com.example.demo" versionCode=42 versionName="1.2.3">
	var elemChunk bytes.Buffer
	attrCount := uint16(3)
	elemChunkSize := uint32(36 + int(attrCount)*20)
	_ = binary.Write(&elemChunk, binary.LittleEndian, uint16(0x0102))
	_ = binary.Write(&elemChunk, binary.LittleEndian, uint16(16))
	_ = binary.Write(&elemChunk, binary.LittleEndian, elemChunkSize)
	_ = binary.Write(&elemChunk, binary.LittleEndian, uint32(1))
	_ = binary.Write(&elemChunk, binary.LittleEndian, uint32(0xFFFFFFFF))
	_ = binary.Write(&elemChunk, binary.LittleEndian, uint32(0xFFFFFFFF))
	_ = binary.Write(&elemChunk, binary.LittleEndian, uint32(0)) // "manifest"
	_ = binary.Write(&elemChunk, binary.LittleEndian, uint16(20))
	_ = binary.Write(&elemChunk, binary.LittleEndian, uint16(20))
	_ = binary.Write(&elemChunk, binary.LittleEndian, attrCount)
	_ = binary.Write(&elemChunk, binary.LittleEndian, uint16(0))
	_ = binary.Write(&elemChunk, binary.LittleEndian, uint16(0))
	_ = binary.Write(&elemChunk, binary.LittleEndian, uint16(0))

	writeAttr := func(nameIdx, rawValIdx uint32, dataType uint8, dataVal uint32) {
		_ = binary.Write(&elemChunk, binary.LittleEndian, uint32(0xFFFFFFFF))
		_ = binary.Write(&elemChunk, binary.LittleEndian, nameIdx)
		_ = binary.Write(&elemChunk, binary.LittleEndian, rawValIdx)
		_ = binary.Write(&elemChunk, binary.LittleEndian, uint16(8))
		_ = binary.Write(&elemChunk, binary.LittleEndian, uint8(0))
		_ = binary.Write(&elemChunk, binary.LittleEndian, dataType)
		_ = binary.Write(&elemChunk, binary.LittleEndian, dataVal)
	}
	writeAttr(1, 2, 0x03, 2)           // package="com.example.demo"
	writeAttr(3, 0xFFFFFFFF, 0x10, 42) // versionCode=42
	writeAttr(4, 5, 0x03, 5)           // versionName="1.2.3"

	totalSize := uint32(8 + poolChunk.Len() + elemChunk.Len())
	var doc bytes.Buffer
	_ = binary.Write(&doc, binary.LittleEndian, uint16(0x0003))
	_ = binary.Write(&doc, binary.LittleEndian, uint16(8))
	_ = binary.Write(&doc, binary.LittleEndian, totalSize)
	doc.Write(poolChunk.Bytes())
	doc.Write(elemChunk.Bytes())
	return doc.Bytes()
}

func TestInspectArchive_StandardAPK(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w1, _ := zw.Create("AndroidManifest.xml")
	_, _ = w1.Write(buildSyntheticAXML())
	w2, _ := zw.Create("classes.dex")
	_, _ = w2.Write([]byte("dex"))
	w3, _ := zw.Create("lib/arm64-v8a/libnative.so")
	_, _ = w3.Write([]byte("elf"))
	w4, _ := zw.Create("META-INF/CERT.RSA")
	_, _ = w4.Write([]byte("sig"))
	_ = zw.Close()

	apkFile := filepath.Join(t.TempDir(), "demo.apk")
	if err := os.WriteFile(apkFile, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write synthetic apk: %v", err)
	}

	info, err := InspectArchive(apkFile)
	if err != nil {
		t.Fatalf("InspectArchive failed: %v", err)
	}

	if info.PackageName != "com.example.demo" || info.VersionCode != 42 || info.VersionName != "1.2.3" {
		t.Errorf("unexpected manifest metadata: %+v", info)
	}
	if info.DexCount != 1 || len(info.ABIs) != 1 || info.ABIs[0] != "arm64-v8a" {
		t.Errorf("unexpected binary entries: %+v", info)
	}
}
