package pbsbackup

import (
	"encoding/binary"
	"fmt"
	"strings"

	"pbs-win-backup/internal/i18nconfig"

	"github.com/dchest/siphash"
)

const (
	pxarGoodbyeTail = uint64(0xef5eed5b753e1555)
	sipHashK0       = uint64(0x83ac3f1cfbb450db)
	sipHashK1       = uint64(0xaa4f1b6879369fbd)
)

type goodbyeItem struct {
	hash   uint64
	offset uint64
	length uint64
}

func pxarNameHash(name string) uint64 {
	return siphash.Hash(sipHashK0, sipHashK1, []byte(name))
}

func parseGoodbyeBlock(data []byte, pos int) ([]goodbyeItem, int, error) {
	if pos+16 > len(data) {
		return nil, pos, fmt.Errorf("goodbye header")
	}
	if binary.LittleEndian.Uint64(data[pos:]) != pxarGoodbye {
		return nil, pos, i18nconfig.FromConfig().E("pxar.goodbye.expected")
	}
	blockLen := int(binary.LittleEndian.Uint64(data[pos+8:]))
	end := pos + blockLen
	if end > len(data) || blockLen < 40 {
		return nil, pos, fmt.Errorf("goodbye data")
	}
	body := data[pos+16 : end]
	if len(body)%24 != 0 {
		return nil, pos, fmt.Errorf("goodbye items")
	}
	items := make([]goodbyeItem, 0, len(body)/24)
	for i := 0; i+24 <= len(body); i += 24 {
		items = append(items, goodbyeItem{
			hash:   binary.LittleEndian.Uint64(body[i:]),
			offset: binary.LittleEndian.Uint64(body[i+8:]),
			length: binary.LittleEndian.Uint64(body[i+16:]),
		})
	}
	return items, end, nil
}

func goodbyeBSTFind(items []goodbyeItem, hash uint64) (int, bool) {
	n := len(items)
	if n > 0 && items[n-1].hash == pxarGoodbyeTail {
		n--
	}
	idx := 0
	for idx < n {
		item := items[idx]
		if item.hash == hash {
			return idx, true
		}
		if hash < item.hash {
			idx = 2*idx + 1
		} else {
			idx = 2*idx + 2
		}
	}
	return 0, false
}

func scanToGoodbye(data []byte, pos int) (int, error) {
	for pos < len(data) {
		if pos+8 > len(data) {
			return pos, fmt.Errorf("scan goodbye EOF")
		}
		hdr := binary.LittleEndian.Uint64(data[pos:])
		if hdr == pxarGoodbye {
			return pos, nil
		}
		if hdr != pxarFilename {
			return pos, fmt.Errorf("scan goodbye: block 0x%x", hdr)
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
	return pos, i18nconfig.FromConfig().E("pxar.goodbye.not_found")
}

func lookupChildStart(data []byte, contentPos int, name string) (int, error) {
	goodbyePos, err := scanToGoodbye(data, contentPos)
	if err != nil {
		return 0, err
	}
	items, _, err := parseGoodbyeBlock(data, goodbyePos)
	if err != nil {
		return 0, err
	}
	hash := pxarNameHash(name)
	idx, ok := goodbyeBSTFind(items, hash)
	if !ok {
		return 0, i18nconfig.FromConfig().Ef("pxar.goodbye.file_not_found", map[string]string{"name": name})
	}
	childStart := goodbyePos - int(items[idx].offset)
	if childStart < 0 || childStart+16 > len(data) {
		return 0, i18nconfig.FromConfig().Ef("pxar.goodbye.offset_for", map[string]string{"name": name})
	}
	if binary.LittleEndian.Uint64(data[childStart:]) != pxarFilename {
		return 0, i18nconfig.FromConfig().Ef("pxar.goodbye.invalid_pos", map[string]string{"name": name})
	}
	return childStart, nil
}

func findInPXAR(data []byte, pos int, cur, target []string) ([]byte, error) {
	if len(target) == 0 {
		return nil, i18nconfig.FromConfig().E("path.empty")
	}
	if len(cur) == 0 && pos == 0 {
		var err error
		pos, err = skipEntry(data, pos)
		if err != nil {
			return nil, err
		}
	}

	name := target[0]
	rest := target[1:]

	childPos, err := lookupChildStart(data, pos, name)
	if err != nil {
		return nil, err
	}

	childName, pos, err := readFilename(data, childPos)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(childName, name) {
		return nil, i18nconfig.FromConfig().Ef("pxar.goodbye.name_mismatch", map[string]string{"got": childName, "want": name})
	}
	mode, pos, err := readEntry(data, pos)
	if err != nil {
		return nil, err
	}
	isDir := mode&0o170000 == pxarIFDIR

	if len(rest) == 0 {
		if isDir {
			return nil, i18nconfig.FromConfig().Ef("pxar.goodbye.is_directory", map[string]string{"path": strings.Join(append(cur, name), `\`)})
		}
		return readPayload(data, pos)
	}
	if !isDir {
		return nil, i18nconfig.FromConfig().Ef("pxar.goodbye.not_directory", map[string]string{"path": strings.Join(append(cur, name), `\`)})
	}
	return findInPXAR(data, pos, append(cur, name), rest)
}
