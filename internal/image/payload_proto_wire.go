package image

import (
	"encoding/binary"
	"fmt"
	"math"
)

func walkProtoFields(data []byte, fn func(fieldNum, wireType int, val uint64, raw []byte) error) error {
	pos := 0
	for pos < len(data) {
		tag, n := binary.Uvarint(data[pos:])
		if n <= 0 {
			return fmt.Errorf("invalid protobuf tag at offset %d", pos)
		}
		pos += n
		fieldNumU := tag >> 3
		if fieldNumU == 0 || fieldNumU > math.MaxInt32 {
			return fmt.Errorf("invalid protobuf field number %d at offset %d", fieldNumU, pos-n)
		}
		fieldNum := int(fieldNumU)
		wireType := int(tag & 0x7)

		switch wireType {
		case wireVarint:
			val, vn := binary.Uvarint(data[pos:])
			if vn <= 0 {
				return fmt.Errorf("invalid protobuf varint at offset %d", pos)
			}
			pos += vn
			if err := fn(fieldNum, wireType, val, nil); err != nil {
				return err
			}
		case wireFixed64:
			if pos+8 > len(data) {
				return fmt.Errorf("truncated fixed64 at offset %d", pos)
			}
			pos += 8
		case wireBytes:
			length, ln := binary.Uvarint(data[pos:])
			if ln <= 0 {
				return fmt.Errorf("invalid protobuf length at offset %d", pos)
			}
			pos += ln
			if length > uint64(len(data)-pos) {
				return fmt.Errorf("truncated protobuf bytes at offset %d", pos)
			}
			raw := data[pos : pos+int(length)]
			pos += int(length)
			if err := fn(fieldNum, wireType, 0, raw); err != nil {
				return err
			}
		case wireFixed32:
			if pos+4 > len(data) {
				return fmt.Errorf("truncated fixed32 at offset %d", pos)
			}
			pos += 4
		default:
			return fmt.Errorf("unsupported protobuf wire type %d at offset %d", wireType, pos)
		}
	}
	return nil
}
