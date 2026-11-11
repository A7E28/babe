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
	want := []string{"com.android.settings", "org.mozilla.firefox", "com.google.android.gms"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParsePackageList() = %v, want %v", got, want)
	}
}

func TestParsePackagePaths(t *testing.T) {
	raw := "package:/data/app/~~abc123==/org.mozilla.firefox-xyz==/base.apk\r\npackage:/data/app/~~abc123==/org.mozilla.firefox-xyz==/split_config.arm64_v8a.apk\n"
	gotPaths := ParsePackagePaths(raw)
	wantPaths := []string{
		"/data/app/~~abc123==/org.mozilla.firefox-xyz==/base.apk",
		"/data/app/~~abc123==/org.mozilla.firefox-xyz==/split_config.arm64_v8a.apk",
	}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Errorf("ParsePackagePaths() = %v, want %v", gotPaths, wantPaths)
	}
	if first := ParsePackagePath(raw); first != wantPaths[0] {
		t.Errorf("ParsePackagePath() = %q, want %q", first, wantPaths[0])
	}
}

func buildSyntheticAXML() []byte {
	strs := []string{
		"manifest", "package", "com.example.demo", "versionCode", "versionName", "1.2.3",
		"uses-sdk", "minSdkVersion", "34", "targetSdkVersion", "VanillaIceCream",
	}
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
	stringsStart := uint32(poolHeaderSize + len(strs)*4)
	poolChunkSize := uint32(int(stringsStart) + strData.Len())

	var poolChunk bytes.Buffer
	_ = binary.Write(&poolChunk, binary.LittleEndian, []uint16{0x0001, uint16(poolHeaderSize)})
	_ = binary.Write(&poolChunk, binary.LittleEndian, []uint32{poolChunkSize, uint32(len(strs)), 0, 1 << 8, stringsStart, 0})
	_ = binary.Write(&poolChunk, binary.LittleEndian, offsets)
	poolChunk.Write(strData.Bytes())

	writeElem := func(nameIdx uint32, attrs [][4]uint32) []byte {
		var elem bytes.Buffer
		attrCount := uint16(len(attrs))
		_ = binary.Write(&elem, binary.LittleEndian, []uint16{0x0102, 16})
		_ = binary.Write(&elem, binary.LittleEndian, []uint32{uint32(36 + int(attrCount)*20), 1, 0xFFFFFFFF, 0xFFFFFFFF, nameIdx})
		_ = binary.Write(&elem, binary.LittleEndian, []uint16{20, 20, attrCount, 0, 0, 0})
		for _, a := range attrs {
			_ = binary.Write(&elem, binary.LittleEndian, []uint32{0xFFFFFFFF, a[0], a[1]})
			_ = binary.Write(&elem, binary.LittleEndian, uint16(8))
			_ = binary.Write(&elem, binary.LittleEndian, []uint8{0, uint8(a[2])})
			_ = binary.Write(&elem, binary.LittleEndian, a[3])
		}
		return elem.Bytes()
	}

	manifestElem := writeElem(0, [][4]uint32{
		{1, 2, 0x03, 2},           // package="com.example.demo"
		{3, 0xFFFFFFFF, 0x10, 42}, // versionCode=42
		{4, 5, 0x03, 5},           // versionName="1.2.3"
	})
	usesSDKElem := writeElem(6, [][4]uint32{
		{7, 8, 0x03, 8},   // minSdkVersion="34" stored as string reference at pool index 8
		{9, 10, 0x03, 10}, // targetSdkVersion="VanillaIceCream" stored as string reference at pool index 10
	})

	totalSize := uint32(8 + poolChunk.Len() + len(manifestElem) + len(usesSDKElem))
	var doc bytes.Buffer
	_ = binary.Write(&doc, binary.LittleEndian, []uint16{0x0003, 8})
	_ = binary.Write(&doc, binary.LittleEndian, totalSize)
	doc.Write(poolChunk.Bytes())
	doc.Write(manifestElem)
	doc.Write(usesSDKElem)
	return doc.Bytes()
}

func createTestZip(entries map[string][]byte) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range entries {
		w, _ := zw.Create(name)
		_, _ = w.Write(data)
	}
	_ = zw.Close()
	return buf.Bytes()
}

func TestInspectArchive_StandardAPKAndStringSDK(t *testing.T) {
	apkBytes := createTestZip(map[string][]byte{
		"AndroidManifest.xml":        buildSyntheticAXML(),
		"classes.dex":                []byte("dex"),
		"lib/arm64-v8a/libnative.so": []byte("elf"),
		"META-INF/CERT.RSA":          []byte("sig"),
	})

	apkFile := filepath.Join(t.TempDir(), "demo.apk")
	if err := os.WriteFile(apkFile, apkBytes, 0644); err != nil {
		t.Fatalf("write synthetic apk: %v", err)
	}

	info, err := InspectArchive(apkFile)
	if err != nil {
		t.Fatalf("InspectArchive failed: %v", err)
	}
	if info.PackageName != "com.example.demo" || info.VersionCode != 42 || info.VersionName != "1.2.3" {
		t.Errorf("unexpected manifest metadata: %+v", info)
	}
	if info.MinSDK != 34 {
		t.Errorf("expected MinSDK=34 (decoded from string pool), got %d", info.MinSDK)
	}
	if info.TargetSDK != 0 || info.TargetSDKCodename != "VanillaIceCream" {
		t.Errorf("expected TargetSDK=0 and TargetSDKCodename=VanillaIceCream, got %d / %q", info.TargetSDK, info.TargetSDKCodename)
	}
	if info.DexCount != 1 || len(info.ABIs) != 1 || info.ABIs[0] != "arm64-v8a" {
		t.Errorf("unexpected binary entries: %+v", info)
	}
}

func TestPackAndInspectSplitBundle(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "base.apk")
	_ = os.WriteFile(basePath, createTestZip(map[string][]byte{
		"AndroidManifest.xml": buildSyntheticAXML(),
		"classes.dex":         []byte("dex"),
		"META-INF/CERT.RSA":   []byte("sig"),
	}), 0644)

	splitPath := filepath.Join(tmpDir, "split_config.arm64_v8a.apk")
	_ = os.WriteFile(splitPath, createTestZip(map[string][]byte{
		"lib/arm64-v8a/libapp.so": []byte("elf"),
	}), 0644)

	bundlePath := filepath.Join(tmpDir, "bundled.apks")
	if err := PackSplitBundle([]string{basePath, splitPath}, bundlePath); err != nil {
		t.Fatalf("PackSplitBundle failed: %v", err)
	}

	info, err := InspectArchive(bundlePath)
	if err != nil {
		t.Fatalf("InspectArchive on bundle failed: %v", err)
	}
	if !info.IsBundle || len(info.SplitAPKs) != 2 || info.PackageName != "com.example.demo" || info.DexCount != 1 {
		t.Errorf("unexpected bundle metadata: %+v", info)
	}
	if len(info.ABIs) != 1 || info.ABIs[0] != "arm64-v8a" {
		t.Errorf("expected arm64-v8a ABI from nested split APK, got %v", info.ABIs)
	}
}
