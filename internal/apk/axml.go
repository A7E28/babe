package apk

import (
	"encoding/binary"
	"strconv"
	"unicode/utf16"
)

const (
	resStrPoolType    = 0x0001
	resXMLResMapType  = 0x0180
	resXMLStartElType = 0x0102
)

// ParseAXMLManifest extracts package, version, SDK, and permission attributes from binary AndroidManifest.xml.
func ParseAXMLManifest(data []byte, info *ArchiveInfo) {
	if len(data) < 8 || binary.LittleEndian.Uint16(data[0:2]) != 0x0003 {
		return
	}
	var stringsPool []string
	var resIDs []uint32
	offset := int(binary.LittleEndian.Uint16(data[2:4]))

	for offset+8 <= len(data) {
		chunkType := binary.LittleEndian.Uint16(data[offset : offset+2])
		headerSize := int(binary.LittleEndian.Uint16(data[offset+2 : offset+4]))
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		if chunkSize <= 0 || offset+chunkSize > len(data) {
			break
		}
		chunk := data[offset : offset+chunkSize]
		switch chunkType {
		case resStrPoolType:
			stringsPool = parseStringPool(chunk, headerSize)
		case resXMLResMapType:
			resIDs = parseResourceMap(chunk, headerSize)
		case resXMLStartElType:
			parseStartElement(chunk, stringsPool, resIDs, info)
		}
		offset += chunkSize
	}
}

func parseResourceMap(chunk []byte, headerSize int) []uint32 {
	if headerSize < 8 || len(chunk) < headerSize {
		return nil
	}
	ids := make([]uint32, (len(chunk)-headerSize)/4)
	for i := range ids {
		pos := headerSize + i*4
		ids[i] = binary.LittleEndian.Uint32(chunk[pos : pos+4])
	}
	return ids
}

func parseStringPool(chunk []byte, headerSize int) []string {
	if len(chunk) < 28 {
		return nil
	}
	stringCount := int(binary.LittleEndian.Uint32(chunk[8:12]))
	isUTF8 := (binary.LittleEndian.Uint32(chunk[16:20]) & (1 << 8)) != 0
	stringsStart := int(binary.LittleEndian.Uint32(chunk[20:24]))

	pool := make([]string, stringCount)
	for i := 0; i < stringCount; i++ {
		idxPos := headerSize + i*4
		if idxPos+4 > len(chunk) {
			break
		}
		strOffset := stringsStart + int(binary.LittleEndian.Uint32(chunk[idxPos:idxPos+4]))
		if strOffset >= 0 && strOffset < len(chunk) {
			pool[i] = decodePoolString(chunk[strOffset:], isUTF8)
		}
	}
	return pool
}

func decodePoolString(b []byte, isUTF8 bool) string {
	if len(b) < 2 {
		return ""
	}
	if isUTF8 {
		pos := 1
		if b[0]&0x80 != 0 {
			pos = 2
		}
		if pos >= len(b) {
			return ""
		}
		byteLen := int(b[pos])
		pos++
		if byteLen&0x80 != 0 && pos < len(b) {
			byteLen = ((byteLen & 0x7F) << 8) | int(b[pos])
			pos++
		}
		if pos+byteLen > len(b) {
			return ""
		}
		return string(b[pos : pos+byteLen])
	}
	charLen, pos := int(binary.LittleEndian.Uint16(b[0:2])), 2
	if charLen&0x8000 != 0 && len(b) >= 4 {
		charLen, pos = ((charLen&0x7FFF)<<16)|int(binary.LittleEndian.Uint16(b[2:4])), 4
	}
	if pos+charLen*2 > len(b) {
		return ""
	}
	u16 := make([]uint16, charLen)
	for i := 0; i < charLen; i++ {
		u16[i] = binary.LittleEndian.Uint16(b[pos+i*2 : pos+(i+1)*2])
	}
	return string(utf16.Decode(u16))
}

func parseStartElement(chunk []byte, pool []string, resIDs []uint32, info *ArchiveInfo) {
	if len(chunk) < 36 {
		return
	}
	elemName := poolStr(pool, int(int32(binary.LittleEndian.Uint32(chunk[20:24]))))
	attrStart := int(binary.LittleEndian.Uint16(chunk[24:26]))
	attrSize := max(int(binary.LittleEndian.Uint16(chunk[26:28])), 20)
	attrCount := int(binary.LittleEndian.Uint16(chunk[28:30]))

	for i, pos := 0, 16+attrStart; i < attrCount && pos+20 <= len(chunk); i, pos = i+1, pos+attrSize {
		attrName := resolveAttrName(pool, resIDs, int(int32(binary.LittleEndian.Uint32(chunk[pos+4:pos+8]))))
		strVal := poolStr(pool, int(int32(binary.LittleEndian.Uint32(chunk[pos+8:pos+12]))))
		dataType, dataVal := chunk[pos+15], binary.LittleEndian.Uint32(chunk[pos+16:pos+20])
		if strVal == "" && dataType == 0x03 {
			strVal = poolStr(pool, int(int32(dataVal)))
		}
		applyManifestAttr(elemName, attrName, dataType, dataVal, strVal, info)
	}
}

func applyManifestAttr(elemName, attrName string, dataType uint8, dataVal uint32, strVal string, info *ArchiveInfo) {
	switch elemName {
	case "manifest":
		switch attrName {
		case "package":
			info.PackageName = strVal
		case "versionCode":
			if v, _ := resolveUint32Attr(dataType, dataVal, strVal); v > 0 {
				info.VersionCode = v
			}
		case "versionName":
			info.VersionName = strVal
		}
	case "uses-sdk":
		switch attrName {
		case "minSdkVersion":
			info.MinSDK, info.MinSDKCodename = resolveUint32Attr(dataType, dataVal, strVal)
		case "targetSdkVersion":
			info.TargetSDK, info.TargetSDKCodename = resolveUint32Attr(dataType, dataVal, strVal)
		}
	case "uses-permission", "uses-permission-sdk-23":
		if attrName == "name" && strVal != "" {
			info.Permissions = append(info.Permissions, strVal)
		}
	}
}

var resAttrNames = map[uint32]string{
	0x01010003: "name",
	0x0101020c: "minSdkVersion",
	0x0101021b: "versionCode",
	0x0101021c: "versionName",
	0x01010270: "targetSdkVersion",
}

func resolveAttrName(pool []string, resIDs []uint32, idx int) string {
	if idx >= 0 && idx < len(resIDs) {
		if name, ok := resAttrNames[resIDs[idx]]; ok {
			return name
		}
	}
	return poolStr(pool, idx)
}

func resolveUint32Attr(dataType uint8, dataVal uint32, strVal string) (uint32, string) {
	if dataType == 0x10 || dataType == 0x11 {
		return dataVal, ""
	}
	if strVal != "" || dataType == 0x03 {
		if n, err := strconv.ParseUint(strVal, 10, 32); err == nil {
			return uint32(n), ""
		}
		return 0, strVal
	}
	return dataVal, ""
}

func poolStr(pool []string, idx int) string {
	if idx >= 0 && idx < len(pool) {
		return pool[idx]
	}
	return ""
}
