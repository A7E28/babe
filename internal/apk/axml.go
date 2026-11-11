package apk

import (
	"encoding/binary"
	"unicode/utf16"
)

// ParseAXMLManifest extracts package, version, SDK, and permission attributes from binary AndroidManifest.xml.
func ParseAXMLManifest(data []byte, info *ArchiveInfo) {
	if len(data) < 8 || binary.LittleEndian.Uint16(data[0:2]) != 0x0003 {
		return
	}

	var stringsPool []string
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
		case 0x0001: // RES_STRING_POOL_TYPE
			stringsPool = parseStringPool(chunk, headerSize)
		case 0x0102: // RES_XML_START_ELEMENT_TYPE
			parseStartElement(chunk, stringsPool, info)
		}
		offset += chunkSize
	}
}

func parseStringPool(chunk []byte, headerSize int) []string {
	if len(chunk) < 28 {
		return nil
	}
	stringCount := int(binary.LittleEndian.Uint32(chunk[8:12]))
	flags := binary.LittleEndian.Uint32(chunk[16:20])
	stringsStart := int(binary.LittleEndian.Uint32(chunk[20:24]))
	isUTF8 := (flags & (1 << 8)) != 0

	pool := make([]string, stringCount)
	for i := 0; i < stringCount; i++ {
		idxPos := headerSize + i*4
		if idxPos+4 > len(chunk) {
			break
		}
		strOffset := stringsStart + int(binary.LittleEndian.Uint32(chunk[idxPos:idxPos+4]))
		if strOffset < 0 || strOffset >= len(chunk) {
			continue
		}
		pool[i] = decodePoolString(chunk[strOffset:], isUTF8)
	}
	return pool
}

func decodePoolString(b []byte, isUTF8 bool) string {
	if isUTF8 {
		if len(b) < 2 {
			return ""
		}
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

	if len(b) < 2 {
		return ""
	}
	charLen := int(binary.LittleEndian.Uint16(b[0:2]))
	pos := 2
	if charLen&0x8000 != 0 && len(b) >= 4 {
		charLen = ((charLen & 0x7FFF) << 16) | int(binary.LittleEndian.Uint16(b[2:4]))
		pos = 4
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

func parseStartElement(chunk []byte, pool []string, info *ArchiveInfo) {
	if len(chunk) < 36 {
		return
	}
	nameIdx := int(int32(binary.LittleEndian.Uint32(chunk[20:24])))
	attrStart := int(binary.LittleEndian.Uint16(chunk[24:26]))
	attrSize := int(binary.LittleEndian.Uint16(chunk[26:28]))
	attrCount := int(binary.LittleEndian.Uint16(chunk[28:30]))
	if attrSize < 20 {
		attrSize = 20
	}

	elemName := poolStr(pool, nameIdx)
	pos := 16 + attrStart

	for i := 0; i < attrCount && pos+20 <= len(chunk); i++ {
		attrName := poolStr(pool, int(int32(binary.LittleEndian.Uint32(chunk[pos+4:pos+8]))))
		rawValIdx := int(int32(binary.LittleEndian.Uint32(chunk[pos+8:pos+12])))
		dataType := chunk[pos+15]
		dataVal := binary.LittleEndian.Uint32(chunk[pos+16 : pos+20])

		strVal := poolStr(pool, rawValIdx)
		if strVal == "" && dataType == 0x03 {
			strVal = poolStr(pool, int(int32(dataVal)))
		}

		switch elemName {
		case "manifest":
			switch attrName {
			case "package":
				info.PackageName = strVal
			case "versionCode":
				info.VersionCode = dataVal
			case "versionName":
				info.VersionName = strVal
			}
		case "uses-sdk":
			switch attrName {
			case "minSdkVersion":
				info.MinSDK = dataVal
			case "targetSdkVersion":
				info.TargetSDK = dataVal
			}
		case "uses-permission":
			if attrName == "name" && strVal != "" {
				info.Permissions = append(info.Permissions, strVal)
			}
		}
		pos += attrSize
	}
}

func poolStr(pool []string, idx int) string {
	if idx >= 0 && idx < len(pool) {
		return pool[idx]
	}
	return ""
}
