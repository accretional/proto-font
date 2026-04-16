package fontcodec

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/andybalholm/brotli"

	pb "openformat/gen/go/openformat/v1"
)

// woff2KnownTags is the WOFF2 spec §5 well-known tag table. The flags
// byte's low 6 bits index into this slice when value is < 63; value 63
// signals an explicit uint32 tag follows in the wire format.
var woff2KnownTags = [63]string{
	"cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post",
	"cvt ", "fpgm", "glyf", "loca", "prep", "CFF ", "VORG", "EBDT",
	"EBLC", "gasp", "hdmx", "kern", "LTSH", "PCLT", "VDMX", "vhea",
	"vmtx", "BASE", "GDEF", "GPOS", "GSUB", "EBSC", "JSTF", "MATH",
	"CBDT", "CBLC", "COLR", "CPAL", "SVG ", "sbix", "acnt", "avar",
	"bdat", "bloc", "bsln", "cvar", "fdsc", "feat", "fmtx", "fvar",
	"gvar", "hsty", "just", "lcar", "mort", "morx", "opbd", "prop",
	"trak", "Zapf", "Silf", "Glat", "Gloc", "Feat", "Sill",
}

// readUIntBase128 decodes one variable-length 32-bit unsigned integer
// per W3C WOFF2 spec §6.1.1. Used for origLength / transformLength
// fields in the WOFF2 table directory. Returns the value and the
// number of bytes consumed.
func readUIntBase128(buf []byte) (uint32, int, error) {
	var accum uint32
	for i := 0; i < 5; i++ {
		if i >= len(buf) {
			return 0, 0, io.ErrUnexpectedEOF
		}
		b := buf[i]
		if i == 0 && b == 0x80 {
			return 0, 0, fmt.Errorf("UIntBase128: leading zero byte")
		}
		if accum&0xFE000000 != 0 {
			return 0, 0, fmt.Errorf("UIntBase128: overflow")
		}
		accum = (accum << 7) | uint32(b&0x7f)
		if b&0x80 == 0 {
			return accum, i + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("UIntBase128: more than 5 bytes")
}

// writeUIntBase128 emits the minimal big-endian UIntBase128 encoding of v
// (spec §6.1.1). Each byte carries 7 bits; the most-significant byte has
// no continuation-bit set only on the final byte.
func writeUIntBase128(v uint32) []byte {
	if v == 0 {
		return []byte{0}
	}
	var tmp [5]byte
	n := 0
	for x := v; x > 0; x >>= 7 {
		tmp[n] = byte(x & 0x7f)
		n++
	}
	out := make([]byte, n)
	for i := 0; i < n; i++ {
		out[i] = tmp[n-1-i]
		if i < n-1 {
			out[i] |= 0x80
		}
	}
	return out
}

// write255UShort emits the minimal 255UInt16 encoding of v per spec §6.1.1.
func write255UShort(v uint32) []byte {
	switch {
	case v < 253:
		return []byte{byte(v)}
	case v < 253+253:
		return []byte{255, byte(v - 253)}
	case v < 253+253+253:
		return []byte{254, byte(v - 506)}
	default:
		return []byte{253, byte(v >> 8), byte(v)}
	}
}

// read255UShort decodes one variable-length integer per W3C WOFF2 spec
// §6.1.1. Returns the value and the number of bytes consumed.
func read255UShort(buf []byte) (uint32, int, error) {
	if len(buf) == 0 {
		return 0, 0, io.ErrUnexpectedEOF
	}
	switch buf[0] {
	case 253:
		if len(buf) < 3 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		return uint32(binary.BigEndian.Uint16(buf[1:3])), 3, nil
	case 254:
		if len(buf) < 2 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		return uint32(buf[1]) + 506, 2, nil
	case 255:
		if len(buf) < 2 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		return uint32(buf[1]) + 253, 2, nil
	default:
		return uint32(buf[0]), 1, nil
	}
}

// hasWoff2Transform decides whether a table directory entry stores
// transformed bytes (and therefore carries a transformLength field) per
// WOFF2 spec §5.1: glyf/loca are transformed unless transformVersion==3,
// every other tag is transformed only when transformVersion!=0.
func hasWoff2Transform(tag string, transformVersion uint8) bool {
	if tag == "glyf" || tag == "loca" {
		return transformVersion != 3
	}
	return transformVersion != 0
}

// parseWoff2Directory walks numTables variable-length directory entries
// starting at buf[0] and returns the populated entries plus the number
// of bytes consumed.
func parseWoff2Directory(buf []byte, numTables uint32) ([]*pb.Woff2TableDirectoryEntry, int, error) {
	out := make([]*pb.Woff2TableDirectoryEntry, 0, numTables)
	cursor := 0
	for i := uint32(0); i < numTables; i++ {
		if cursor >= len(buf) {
			return nil, 0, fmt.Errorf("woff2: directory truncated at entry %d", i)
		}
		flags := buf[cursor]
		cursor++

		tagIdx := flags & 0x3f
		transformVer := flags >> 6

		var tagWire uint32
		if tagIdx == 63 {
			if cursor+4 > len(buf) {
				return nil, 0, fmt.Errorf("woff2: directory truncated reading explicit tag at entry %d", i)
			}
			tagWire = binary.BigEndian.Uint32(buf[cursor : cursor+4])
			cursor += 4
		} else {
			tagWire = packKnownTag(woff2KnownTags[tagIdx])
		}
		tagStr := tagString(tagWire)

		origLen, n, err := readUIntBase128(buf[cursor:])
		if err != nil {
			return nil, 0, fmt.Errorf("woff2: entry %d origLength: %w", i, err)
		}
		cursor += n

		entry := &pb.Woff2TableDirectoryEntry{
			Flags:      uint32(flags),
			Tag:        tagWire,
			TagStr:     tagStr,
			OrigLength: origLen,
		}
		if hasWoff2Transform(tagStr, transformVer) {
			tlen, n, err := readUIntBase128(buf[cursor:])
			if err != nil {
				return nil, 0, fmt.Errorf("woff2: entry %d transformLength: %w", i, err)
			}
			cursor += n
			entry.TransformLength = tlen
			entry.Transformed = true
		}
		out = append(out, entry)
	}
	return out, cursor, nil
}

// packKnownTag packs an exactly-four-byte tag from the WOFF2 known-tag
// table into the wire-order uint32 used by SFNT/WOFF directories.
// woff2KnownTags entries are pre-padded with trailing spaces ("cvt ",
// "OS/2", "CFF ", "SVG ") so no implicit padding is needed.
func packKnownTag(s string) uint32 {
	return binary.BigEndian.Uint32([]byte(s))
}

// brotliDecode decompresses a brotli-compressed byte slice.
func brotliDecode(b []byte) ([]byte, error) {
	r := brotli.NewReader(bytes.NewReader(b))
	return io.ReadAll(r)
}

// sliceWoff2TableData carves the brotli-decompressed font data block
// into per-entry payloads, matching the directory order. Per spec §5
// table bodies are concatenated with no padding between them.
func sliceWoff2TableData(decompressed []byte, entries []*pb.Woff2TableDirectoryEntry) error {
	cursor := 0
	for i, e := range entries {
		stored := e.OrigLength
		if e.Transformed {
			stored = e.TransformLength
		}
		end := cursor + int(stored)
		if end > len(decompressed) {
			return fmt.Errorf("woff2: decompressed stream too short for entry %d (%s)", i, e.TagStr)
		}
		e.Data = append([]byte(nil), decompressed[cursor:end]...)
		cursor = end
	}
	return nil
}
