package pbsbackup

import (
	"encoding/binary"
	"fmt"
	"strings"

	"pbs-win-backup/internal/i18nconfig"
)

const (
	pxarFilename = uint64(0x16701121063917b3)
	pxarEntry    = uint64(0xd5956474e588acef)
	pxarPayload  = uint64(0x28147a1b0b7c1a25)
	pxarGoodbye  = uint64(0x2fec4fa642d5731d)
	pxarIFDIR    = uint64(0o0040000)
)

func normalizeRestorePath(p string) string {
	p = strings.ReplaceAll(p, "/", `\`)
	return strings.TrimLeft(p, `\`)
}

func extractFileFromPXAR(pxar []byte, targetPath string) ([]byte, error) {
	parts := strings.Split(normalizeRestorePath(targetPath), `\`)
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	data, err := findInPXAR(pxar, 0, nil, parts)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readFilename(data []byte, pos int) (string, int, error) {
	if pos+16 > len(data) {
		return "", pos, fmt.Errorf("filename header")
	}
	blockLen := int(binary.LittleEndian.Uint64(data[pos+8:]))
	if blockLen < 17 || pos+blockLen > len(data) {
		return "", pos, fmt.Errorf("filename data")
	}
	nameStart := pos + 16
	nameEnd := pos + blockLen - 1
	if data[nameEnd] != 0 {
		return "", pos, fmt.Errorf("filename terminator")
	}
	return string(data[nameStart:nameEnd]), pos + blockLen, nil
}

func readEntry(data []byte, pos int) (uint64, int, error) {
	if pos+16 > len(data) {
		return 0, pos, fmt.Errorf("entry header")
	}
	if binary.LittleEndian.Uint64(data[pos:]) != pxarEntry {
		return 0, pos, i18nconfig.FromConfig().E("pxar.expected_entry")
	}
	entryLen := int(binary.LittleEndian.Uint64(data[pos+8:]))
	if entryLen < 16 || pos+entryLen > len(data) {
		return 0, pos, fmt.Errorf("entry data")
	}
	mode := binary.LittleEndian.Uint64(data[pos+16:])
	return mode, pos + entryLen, nil
}

func skipEntry(data []byte, pos int) (int, error) {
	_, pos, err := readBlock(data, pos)
	return pos, err
}

func readBlock(data []byte, pos int) (uint64, int, error) {
	if pos+16 > len(data) {
		return 0, pos, fmt.Errorf("block header")
	}
	hdr := binary.LittleEndian.Uint64(data[pos:])
	ln := binary.LittleEndian.Uint64(data[pos+8:])
	end := pos + 16 + int(ln) - 16
	if end > len(data) {
		return hdr, pos, fmt.Errorf("block data")
	}
	return hdr, end, nil
}

func readPayload(data []byte, pos int) ([]byte, error) {
	if pos+16 > len(data) {
		return nil, fmt.Errorf("payload header")
	}
	if binary.LittleEndian.Uint64(data[pos:]) != pxarPayload {
		return nil, i18nconfig.FromConfig().E("pxar.expected_payload")
	}
	plen := binary.LittleEndian.Uint64(data[pos+8:])
	start := pos + 16
	end := start + int(plen) - 16
	if end > len(data) {
		return nil, fmt.Errorf("payload data")
	}
	return data[start:end], nil
}

func skipPayload(data []byte, pos int) (int, error) {
	_, pos, err := readBlock(data, pos)
	return pos, err
}

func skipDirectoryContents(data []byte, pos int) (int, error) {
	for pos < len(data) {
		if pos+8 > len(data) {
			return pos, fmt.Errorf("skip dir EOF")
		}
		hdr := binary.LittleEndian.Uint64(data[pos:])
		if hdr == pxarGoodbye {
			_, pos, err := readBlock(data, pos)
			return pos, err
		}
		if hdr != pxarFilename {
			return pos, fmt.Errorf("skip dir: block 0x%x", hdr)
		}
		var err error
		_, pos, err = readFilename(data, pos)
		if err != nil {
			return pos, err
		}
		var mode uint64
		mode, pos, err = readEntry(data, pos)
		if err != nil {
			return pos, err
		}
		if mode&0o170000 == pxarIFDIR {
			pos, err = skipDirectoryContents(data, pos)
		} else {
			pos, err = skipPayload(data, pos)
		}
		if err != nil {
			return pos, err
		}
	}
	return pos, nil
}

