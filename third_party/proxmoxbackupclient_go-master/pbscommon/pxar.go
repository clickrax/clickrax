package pbscommon

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"os"
	"sort"
	"sync/atomic"

	//	"io/ioutil"
	"path/filepath"

	"github.com/dchest/siphash"
)

const (
	pxarSmallFileMax  = 256 * 1024  // read whole file at once
	pxarMediumFileMax = 4 * 1024 * 1024
	pxarReadLarge     = 1 << 20     // 1 MiB for large sequential files
	pxarReadMedium    = 256 * 1024  // 256 KiB
	pxarFlushBatch    = 1 << 20     // batch pxar buffer before streaming to chunks
	pxarFlushScratch  = 256 * 1024  // WriteCB slice size during Flush
)

const (
	PXAR_ENTRY               uint64 = 0xd5956474e588acef
	PXAR_ENTRY_V1            uint64 = 0x11da850a1c1cceff
	PXAR_FILENAME            uint64 = 0x16701121063917b3
	PXAR_SYMLINK             uint64 = 0x27f971e7dbf5dc5f
	PXAR_DEVICE              uint64 = 0x9fc9e906586d5ce9
	PXAR_XATTR               uint64 = 0x0dab0229b57dcd03
	PXAR_ACL_USER            uint64 = 0x2ce8540a457d55b8
	PXAR_ACL_GROUP           uint64 = 0x136e3eceb04c03ab
	PXAR_ACL_GROUP_OBJ       uint64 = 0x10868031e9582876
	PXAR_ACL_DEFAULT         uint64 = 0xbbbb13415a6896f5
	PXAR_ACL_DEFAULT_USER    uint64 = 0xc89357b40532cd1f
	PXAR_ACL_DEFAULT_GROUP   uint64 = 0xf90a8a5816038ffe
	PXAR_FCAPS               uint64 = 0x2da9dd9db5f7fb67
	PXAR_QUOTA_PROJID        uint64 = 0xe07540e82f7d1cbb
	PXAR_HARDLINK            uint64 = 0x51269c8422bd7275
	PXAR_PAYLOAD             uint64 = 0x28147a1b0b7c1a25
	PXAR_GOODBYE             uint64 = 0x2fec4fa642d5731d
	PXAR_GOODBYE_TAIL_MARKER uint64 = 0xef5eed5b753e1555
)

var catalog_magic = []byte{145, 253, 96, 249, 196, 103, 88, 213}

const (
	IFMT   uint64 = 0o0170000
	IFSOCK uint64 = 0o0140000
	IFLNK  uint64 = 0o0120000
	IFREG  uint64 = 0o0100000
	IFBLK  uint64 = 0o0060000
	IFDIR  uint64 = 0o0040000
	IFCHR  uint64 = 0o0020000
	IFIFO  uint64 = 0o0010000

	ISUID uint64 = 0o0004000
	ISGID uint64 = 0o0002000
	ISVTX uint64 = 0o0001000
)

type MTime struct {
	secs    uint64
	nanos   uint32
	padding uint32
}
type PXARFileEntry struct {
	hdr   uint64
	len   uint64
	mode  uint64
	flags uint64
	uid   uint32
	gid   uint32
	mtime MTime
}

type PXARFilenameEntry struct {
	hdr uint64
	len uint64
}

type GoodByeItem struct {
	hash   uint64
	offset uint64
	len    uint64
}

type GoodByeBST struct {
	self  *GoodByeItem
	left  *GoodByeBST
	right *GoodByeBST
}

func (B *GoodByeBST) AddNode(i *GoodByeItem) {
	if i.hash < B.self.hash {
		if B.left == nil {
			B.left = &GoodByeBST{
				self: i,
			}
		} else {
			B.left.AddNode(i)
		}
	}
	if i.hash > B.self.hash {
		if B.right == nil {
			B.right = &GoodByeBST{
				self: i,
			}
		} else {
			B.right.AddNode(i)
		}
	}
}

func pow_of_2(e uint64) uint64 {
	return 1 << e
}

func log_of_2(k uint64) uint64 {
	return 8*8 - uint64(bits.LeadingZeros64(k)) - 1
}

func make_bst_inner(input []GoodByeItem, n uint64, e uint64, output *[]GoodByeItem, i uint64) {
	if n == 0 {
		return
	}
	p := pow_of_2(e - 1)
	q := pow_of_2(e)
	var k uint64
	if n >= p-1+p/2 {
		k = (q - 2) / 2
	} else {
		v := p - 1 + p/2 - n
		k = (q-2)/2 - v
	}

	(*output)[i] = input[k]

	make_bst_inner(input, k, e-1, output, i*2+1)
	make_bst_inner(input[k+1:], n-k-1, e-1, output, i*2+2)
}

func ca_make_bst(input []GoodByeItem, output *[]GoodByeItem) {
	n := uint64(len(input))
	make_bst_inner(input, n, log_of_2(n)+1, output, 0)
}

type PXAROutCB func([]byte)

type PXARArchive struct {
	//Create(filename string, WriteCB PXAROutCB)
	//AddFile(filename string)
	//AddDirectory(dirname string)
	WriteCB        PXAROutCB
	CatalogWriteCB PXAROutCB
	Abort          func() error
	buffer         bytes.Buffer
	pos            uint64
	ArchiveName    string

	catalog_pos uint64

	FilesTotal     *atomic.Int64
	FilesSkipped   *atomic.Int64 // exclusions / unreadable (not fast cache hits)
	FilesFromCache *atomic.Int64 // unchanged files skipped via chunk-span reuse

	ShouldSkip         func(fullPath, name string, isDir bool) bool
	SkipUnreadableDirs bool

	// ReuseFileBytes returns cached PXAR entry bytes for unchanged files (fast incremental).
	ReuseFileBytes  func(path, basename string, fileInfo os.FileInfo) ([]byte, bool)
	ReuseFileChunks func(path, basename string, fileInfo os.FileInfo) ([]PXARFastChunk, bool)
	// OnPxarChunksReuse emits PXAR headers and reuses PBS chunks without reading file data.
	OnPxarChunksReuse func(header []byte, chunks []PXARFastChunk, payloadSize uint64) error
	OnPxarStreamReuse func(blob []byte, chunks []PXARFastChunk) error
	OnFilePxarBegin func()
	OnFilePxarEnd   func(path, basename string, fileInfo os.FileInfo)
}

// PXARFastChunk references an existing PBS chunk in the output stream without re-hashing.
type PXARFastChunk struct {
	DigestHex string
	Len       int
}

func (a *PXARArchive) skipEntry() {
	if a.FilesSkipped != nil {
		a.FilesSkipped.Add(1)
	}
}

func (a *PXARArchive) shouldSkip(fullPath, name string, isDir bool) bool {
	if a.ShouldSkip == nil {
		return false
	}
	return a.ShouldSkip(fullPath, name, isDir)
}

func (a *PXARArchive) checkAbort() error {
	if a.Abort == nil {
		return nil
	}
	return a.Abort()
}

func isAbortErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func readBufferSize(fileSize int64) int {
	switch {
	case fileSize <= 0:
		return pxarReadMedium
	case fileSize <= pxarSmallFileMax:
		return int(fileSize)
	case fileSize <= pxarMediumFileMax:
		return pxarReadMedium
	default:
		return pxarReadLarge
	}
}

//This function will flush the internal buffer and update position
//WriteCB for pxar stream will be called.
//It is useful when we building a data structure and we need to keep a specific offset and output it only at the end

func (a *PXARArchive) Flush() {
	if err := a.checkAbort(); err != nil {
		return
	}
	b := make([]byte, pxarFlushScratch)
	for {
		count, _ := a.buffer.Read(b)
		if count <= 0 {
			break
		}
		a.WriteCB(b[:count])
		a.pos = a.pos + uint64(count)
	}
}

func (a *PXARArchive) flushIfNeeded() {
	if a.buffer.Len() >= pxarFlushBatch {
		a.Flush()
	}
}

func (a *PXARArchive) Create() {
	a.pos = 0
	a.catalog_pos = 8
}

type CatalogDir struct {
	Pos  uint64 //Points to next table so parent has always to be written before children
	Name string
}

type CatalogFile struct {
	Name  string
	MTime uint64
	Size  uint64
}

func append_u64_7bit(a []byte, v uint64) []byte {
	x := a
	for {
		if v < 128 {
			x = append(x, byte(v&0x7f))
			break
		}
		x = append(x, byte(v&0x7f)|byte(0x80))
		v = v >> 7
	}
	return x
}

//PXAR format, documentation had many missing bits i had to figure out
/*
	Suppose we have
	abc
		file.txt
		ced
			file2.txt
			file3.txt

	First entry is always without filename

	PXAR_ENTRY(DIR)
		PXAR_FILENAME(file.txt)
		PXAR_ENTRY(file, attributes etc)
		PXAR_PAYLOAD(file.txt)
		PXAR_FILENAME(ced)
			PXAR_FILENAME(file2.txt)
			PXAR_ENTRY(file,attributes etc)
			PXAR_PAYLOAD(file2.txt)
			PXAR_FILENAME(file3.txt)
			PXAR_ENTRY(file,attributes etc)
			PXAR_PAYLOAD(file3.txt)
			PXAR_GOODBYE( relative to ced
				will have entries sorted using casync algorithms below
				for sip hash of "file2.txt" and "file3.txt", offset is relative to PXAR_GOODBYE header offset
				last special entry with fixed hash and not sorted
			)
		PXAR_GOODBYE(relative to abc or top dir )
			will have entries sorted using casync algorithms below
			for sip hash of "file.txt" and "ced", offset is relative to PXAR_GOODBYE header offset
			last special entry with fixed hash and not sorted
		)

*/

func (a *PXARArchive) WriteDir(path string, dirname string, toplevel bool) (CatalogDir, error) {
	if err := a.checkAbort(); err != nil {
		return CatalogDir{}, err
	}
	if !toplevel && a.shouldSkip(path, dirname, true) {
		a.skipEntry()
		return CatalogDir{Name: dirname, Pos: a.catalog_pos}, nil
	}
	//fmt.Printf("Write dir %s at %d\n", path, a.pos)
	files, err := os.ReadDir(path)
	if err != nil {
		if a.SkipUnreadableDirs && !toplevel {
			a.skipEntry()
			return CatalogDir{Name: dirname, Pos: a.catalog_pos}, nil
		}
		return CatalogDir{}, fmt.Errorf("ReadDir %s: %w", path, err)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return CatalogDir{}, fmt.Errorf("Stat %s: %w", path, err)
	}

	//Avoid writing filename entry on root
	if !toplevel {
		fname_entry := &PXARFilenameEntry{
			hdr: PXAR_FILENAME,
			len: uint64(16) + uint64(len(dirname)) + 1,
		}

		binary.Write(&a.buffer, binary.LittleEndian, fname_entry)

		a.buffer.WriteString(dirname)
		a.buffer.WriteByte(0x00)
	} else {
		if a.CatalogWriteCB != nil {
			a.CatalogWriteCB(catalog_magic)
			a.catalog_pos = 8
		}
	}

	a.Flush()

	dir_start_pos := a.pos

	entry := &PXARFileEntry{
		hdr:   PXAR_ENTRY,
		len:   56,
		mode:  IFDIR | 0o777,
		flags: 0,
		uid:   1000, //This is fixed because this project for now targeting windows , on which execute, traverse etc permissions don't exist
		gid:   1000,
		mtime: MTime{
			secs:    uint64(fileInfo.ModTime().Unix()),
			nanos:   0,
			padding: 0,
		},
	}
	binary.Write(&a.buffer, binary.LittleEndian, entry)

	a.Flush()

	goodbyteitems := make([]GoodByeItem, 0)
	catalog_files := make([]CatalogFile, 0)
	catalog_dirs := make([]CatalogDir, 0)

	for _, file := range files {
		if err := a.checkAbort(); err != nil {
			return CatalogDir{}, err
		}
		childPath := filepath.Join(path, file.Name())
		if a.shouldSkip(childPath, file.Name(), file.IsDir()) {
			a.skipEntry()
			continue
		}
		startpos := a.pos
		if file.IsDir() {

			D, err := a.WriteDir(childPath, file.Name(), false)
			if err != nil {
				if isAbortErr(err) {
					return CatalogDir{}, err
				}
				if a.SkipUnreadableDirs {
					a.skipEntry()
					continue
				}
				return CatalogDir{}, err
			}
			catalog_dirs = append(catalog_dirs, D)
			goodbyteitems = append(goodbyteitems, GoodByeItem{
				offset: startpos,
				hash:   siphash.Hash(0x83ac3f1cfbb450db, 0xaa4f1b6879369fbd, []byte(file.Name())),
				len:    a.pos - startpos,
			})
		} else {
			F, err := a.WriteFile(filepath.Join(path, file.Name()), file.Name())
			if err != nil {
				if isAbortErr(err) {
					return CatalogDir{}, err
				}
				if a.FilesSkipped != nil {
					a.FilesSkipped.Add(1)
				}
				continue
			}

			catalog_files = append(catalog_files, F)
			goodbyteitems = append(goodbyteitems, GoodByeItem{
				offset: startpos,
				hash:   siphash.Hash(0x83ac3f1cfbb450db, 0xaa4f1b6879369fbd, []byte(file.Name())),
				len:    a.pos - startpos,
			})
		}
	}

	//Here we can write AFTER the recursion so leaves get written first
	//We need to write leaves first because otherwise we won't know offsets
	oldpos := a.catalog_pos
	tabledata := make([]byte, 0)
	tabledata = append_u64_7bit(tabledata, uint64(len(catalog_files)+len(catalog_dirs)))
	for _, d := range catalog_dirs {
		tabledata = append(tabledata, 'd')
		tabledata = append_u64_7bit(tabledata, uint64(len(d.Name)))
		tabledata = append(tabledata, []byte(d.Name)...)
		tabledata = append_u64_7bit(tabledata, oldpos-d.Pos)
	}

	for _, f := range catalog_files {
		tabledata = append(tabledata, 'f')
		tabledata = append_u64_7bit(tabledata, uint64(len(f.Name)))
		tabledata = append(tabledata, []byte(f.Name)...)
		tabledata = append_u64_7bit(tabledata, f.Size)
		tabledata = append_u64_7bit(tabledata, f.MTime)
	}

	catalog_outdata := make([]byte, 0)
	catalog_outdata = append_u64_7bit(catalog_outdata, uint64(len(tabledata)))
	catalog_outdata = append(catalog_outdata, tabledata...)

	if a.CatalogWriteCB != nil {
		a.CatalogWriteCB(catalog_outdata)

	}

	a.catalog_pos += uint64(len(catalog_outdata))

	a.Flush()

	//Sort goodbyeitems by sip hash to build later kinda of heap

	sort.Slice(goodbyteitems, func(i, j int) bool {
		return goodbyteitems[i].hash < goodbyteitems[j].hash
	})

	goodbyteitemsnew := make([]GoodByeItem, len(goodbyteitems))

	//Make casync binary search tree structure out of the sorted array

	ca_make_bst(goodbyteitems, &goodbyteitemsnew)

	goodbyteitems = goodbyteitemsnew

	a.Flush()
	goodbye_start := a.pos

	binary.Write(&a.buffer, binary.LittleEndian, PXAR_GOODBYE)
	goodbyelen := uint64(16 + 24*(len(goodbyteitems)+1))
	binary.Write(&a.buffer, binary.LittleEndian, goodbyelen)

	for _, gi := range goodbyteitems {
		gi.offset = a.pos - gi.offset
		binary.Write(&a.buffer, binary.LittleEndian, gi)
	}

	gi := &GoodByeItem{
		offset: goodbye_start - dir_start_pos,
		len:    goodbyelen,
		hash:   0xef5eed5b753e1555,
	}

	binary.Write(&a.buffer, binary.LittleEndian, gi)

	a.Flush()

	if toplevel {
		//We write special pointer to root dir here

		tabledata := make([]byte, 0)
		tabledata = append_u64_7bit(tabledata, uint64(1))
		tabledata = append(tabledata, 'd')
		tabledata = append_u64_7bit(tabledata, uint64(len(a.ArchiveName)))
		tabledata = append(tabledata, []byte(a.ArchiveName)...)
		tabledata = append_u64_7bit(tabledata, a.catalog_pos-oldpos)
		catalog_outdata := make([]byte, 0)
		catalog_outdata = append_u64_7bit(catalog_outdata, uint64(len(tabledata)))
		catalog_outdata = append(catalog_outdata, tabledata...)
		ptr := make([]byte, 0)
		ptr = binary.LittleEndian.AppendUint64(ptr, a.catalog_pos)
		if a.CatalogWriteCB != nil {
			a.CatalogWriteCB(catalog_outdata)
			a.CatalogWriteCB(ptr)
		}
	}

	return CatalogDir{
		Name: dirname,
		Pos:  oldpos,
	}, nil
}

func (a *PXARArchive) buildFileHeaderBytes(basename string, fileInfo os.FileInfo) []byte {
	fnameEntry := &PXARFilenameEntry{
		hdr: PXAR_FILENAME,
		len: uint64(16) + uint64(len(basename)) + 1,
	}
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, fnameEntry)
	_, _ = buf.WriteString(basename)
	_ = buf.WriteByte(0x00)

	entry := &PXARFileEntry{
		hdr:   PXAR_ENTRY,
		len:   56,
		mode:  IFREG | 0o777,
		flags: 0,
		uid:   1000,
		gid:   1000,
		mtime: MTime{
			secs:    uint64(fileInfo.ModTime().Unix()),
			nanos:   0,
			padding: 0,
		},
	}
	_ = binary.Write(&buf, binary.LittleEndian, entry)
	_ = binary.Write(&buf, binary.LittleEndian, PXAR_PAYLOAD)
	filesize := uint64(fileInfo.Size()) + 16
	_ = binary.Write(&buf, binary.LittleEndian, filesize)
	return buf.Bytes()
}

func (a *PXARArchive) tryReuseFileChunks(path, basename string, fileInfo os.FileInfo) (bool, error) {
	if a.ReuseFileChunks == nil {
		return false, nil
	}
	chunks, ok := a.ReuseFileChunks(path, basename, fileInfo)
	if !ok || len(chunks) == 0 {
		return false, nil
	}
	if a.FilesTotal != nil {
		a.FilesTotal.Add(1)
	}
	if a.FilesFromCache != nil {
		a.FilesFromCache.Add(1)
	} else if a.FilesSkipped != nil {
		a.FilesSkipped.Add(1)
	}
	header := a.buildFileHeaderBytes(basename, fileInfo)
	if a.OnPxarChunksReuse != nil {
		if err := a.OnPxarChunksReuse(header, chunks, uint64(fileInfo.Size())); err != nil {
			return false, err
		}
	} else if a.WriteCB != nil {
		a.WriteCB(header)
	}
	return true, nil
}

// On pxar first item and consquently entry point must always be WriteDir , because toplevel is always a directory
// So backing up single file is not possible
func (a *PXARArchive) WriteFile(path string, basename string) (CatalogFile, error) {
	if err := a.checkAbort(); err != nil {
		return CatalogFile{}, err
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		return CatalogFile{}, fmt.Errorf("stat %s: %w", path, err)
	}

	if reused, err := a.tryReuseFileChunks(path, basename, fileInfo); reused || err != nil {
		if err != nil {
			return CatalogFile{}, err
		}
		return CatalogFile{
			Name:  basename,
			MTime: uint64(fileInfo.ModTime().Unix()),
			Size:  uint64(fileInfo.Size()),
		}, nil
	}

	if a.ReuseFileBytes != nil {
		if blob, ok := a.ReuseFileBytes(path, basename, fileInfo); ok && len(blob) > 0 {
			if a.FilesTotal != nil {
				a.FilesTotal.Add(1)
			}
			if a.FilesFromCache != nil {
				a.FilesFromCache.Add(1)
			} else if a.FilesSkipped != nil {
				a.FilesSkipped.Add(1)
			}
			if a.OnPxarStreamReuse != nil && a.ReuseFileChunks != nil {
				if chunks, okChunks := a.ReuseFileChunks(path, basename, fileInfo); okChunks && len(chunks) > 0 {
					if err := a.OnPxarStreamReuse(blob, chunks); err != nil {
						return CatalogFile{}, err
					}
					return CatalogFile{
						Name:  basename,
						MTime: uint64(fileInfo.ModTime().Unix()),
						Size:  uint64(fileInfo.Size()),
					}, nil
				}
			}
			if _, err := a.buffer.Write(blob); err != nil {
				return CatalogFile{}, fmt.Errorf("write cached pxar %s: %w", path, err)
			}
			a.Flush()
			return CatalogFile{
				Name:  basename,
				MTime: uint64(fileInfo.ModTime().Unix()),
				Size:  uint64(fileInfo.Size()),
			}, nil
		}
	}

	if a.OnFilePxarBegin != nil {
		a.OnFilePxarBegin()
	}
	defer func() {
		if a.OnFilePxarEnd != nil {
			a.OnFilePxarEnd(path, basename, fileInfo)
		}
	}()

	//fmt.Printf("Write file %s at %d\n", path, a.pos)
	file, err := openBackupFile(path)

	if err != nil {
		return CatalogFile{}, fmt.Errorf("open %s: %w", path, err)
	}

	defer file.Close()

	if a.FilesTotal != nil {
		a.FilesTotal.Add(1)
	}

	fname_entry := &PXARFilenameEntry{
		hdr: PXAR_FILENAME,
		len: uint64(16) + uint64(len(basename)) + 1,
	}

	binary.Write(&a.buffer, binary.LittleEndian, fname_entry)

	a.buffer.WriteString(basename)
	a.buffer.WriteByte(0x00)

	entry := &PXARFileEntry{
		hdr:   PXAR_ENTRY,
		len:   56,
		mode:  IFREG | 0o777,
		flags: 0,
		uid:   1000,
		gid:   1000,
		mtime: MTime{
			secs:    uint64(fileInfo.ModTime().Unix()),
			nanos:   0,
			padding: 0,
		},
	}
	binary.Write(&a.buffer, binary.LittleEndian, entry)

	binary.Write(&a.buffer, binary.LittleEndian, PXAR_PAYLOAD)
	filesize := uint64(fileInfo.Size()) + 16 //File size + header size
	binary.Write(&a.buffer, binary.LittleEndian, filesize)

	a.Flush()

	readbuffer := make([]byte, readBufferSize(fileInfo.Size()))

	for {
		if err := a.checkAbort(); err != nil {
			return CatalogFile{}, err
		}
		nread, err := file.Read(readbuffer)
		if nread > 0 {
			a.buffer.Write(readbuffer[:nread])
			a.flushIfNeeded()
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return CatalogFile{}, fmt.Errorf("read %s: %w", path, err)
		}
		if nread == 0 {
			break
		}
	}

	a.Flush()

	return CatalogFile{
		Name:  basename,
		MTime: uint64(fileInfo.ModTime().Unix()),
		Size:  uint64(fileInfo.Size()),
	}, nil
}
