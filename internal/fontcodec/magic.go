package fontcodec

import (
	"encoding/binary"
	"errors"

	pb "openformat/gen/go/openformat/v1"
)

const (
	magicTrueType   uint32 = 0x00010000
	magicOpenTypeCF uint32 = 0x4F54544F // 'OTTO'
	magicTrueApple  uint32 = 0x74727565 // 'true'
	magicTyp1       uint32 = 0x74797031 // 'typ1'
	magicWOFF1      uint32 = 0x774F4646 // 'wOFF'
	magicWOFF2      uint32 = 0x774F4632 // 'wOF2'
	magicTTC        uint32 = 0x74746366 // 'ttcf'
	magicEOTMagic   uint16 = 0x504C     // magicNumber in EOT header (at offset 34)
)

var errShortInput = errors.New("fontcodec: input too short for container header")

func detectFlavor(b []byte) (pb.FontContainerFlavor, error) {
	if len(b) < 4 {
		return pb.FontContainerFlavor_FONT_CONTAINER_UNSPECIFIED, errShortInput
	}
	sig := binary.BigEndian.Uint32(b[:4])
	switch sig {
	case magicTrueType:
		return pb.FontContainerFlavor_FONT_CONTAINER_TRUETYPE, nil
	case magicOpenTypeCF:
		return pb.FontContainerFlavor_FONT_CONTAINER_OPENTYPE_CFF, nil
	case magicTrueApple, magicTyp1:
		return pb.FontContainerFlavor_FONT_CONTAINER_TRUE_APPLE, nil
	case magicWOFF1:
		return pb.FontContainerFlavor_FONT_CONTAINER_WOFF1, nil
	case magicWOFF2:
		return pb.FontContainerFlavor_FONT_CONTAINER_WOFF2, nil
	case magicTTC:
		return pb.FontContainerFlavor_FONT_CONTAINER_COLLECTION, nil
	}
	// EOT magic lives inside a little-endian header; spot-check offset 34.
	if len(b) >= 36 && binary.LittleEndian.Uint16(b[34:36]) == magicEOTMagic {
		return pb.FontContainerFlavor_FONT_CONTAINER_EOT, nil
	}
	return pb.FontContainerFlavor_FONT_CONTAINER_UNSPECIFIED, errors.New("fontcodec: unrecognised container magic")
}

// tagString converts a 4-byte table tag to its 4-char ASCII representation.
// Preserves spaces and non-ASCII bytes verbatim.
func tagString(raw uint32) string {
	return string([]byte{
		byte(raw >> 24), byte(raw >> 16), byte(raw >> 8), byte(raw),
	})
}

func tagRaw(s string) uint32 {
	var b [4]byte
	for i := 0; i < 4 && i < len(s); i++ {
		b[i] = s[i]
	}
	return binary.BigEndian.Uint32(b[:])
}
