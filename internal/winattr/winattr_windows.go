//go:build windows

package winattr

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	seFileObject = 1

	ownerSecurityInformation = 0x00000001
	groupSecurityInformation = 0x00000002
	daclSecurityInformation  = 0x00000004

	securityInformation = ownerSecurityInformation | groupSecurityInformation | daclSecurityInformation
)

var (
	advapi32 = windows.NewLazySystemDLL("advapi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetNamedSecurityInfoW = advapi32.NewProc("GetNamedSecurityInfoW")
	procSetFileSecurityW      = advapi32.NewProc("SetFileSecurityW")
	procConvertSDToStringSDW  = advapi32.NewProc("ConvertSecurityDescriptorToStringSecurityDescriptorW")
	procConvertStringSDToSDW = advapi32.NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	procLocalFree             = kernel32.NewProc("LocalFree")

	securityAPIAvailable bool
)

func init() {
	securityAPIAvailable = true
	for _, p := range []*windows.LazyProc{
		procGetNamedSecurityInfoW,
		procSetFileSecurityW,
		procConvertSDToStringSDW,
		procConvertStringSDToSDW,
		procLocalFree,
	} {
		if err := p.Find(); err != nil {
			securityAPIAvailable = false
			return
		}
	}
}

// Capture reads NTFS DACL/owner/group (SDDL) and file attributes.
func Capture(path string) (entry Entry, err error) {
	defer func() {
		if r := recover(); r != nil {
			entry = Entry{}
			err = fmt.Errorf("winattr capture panic: %v", r)
		}
	}()
	if !securityAPIAvailable {
		if entry.HasMeta() {
			return entry, nil
		}
		return entry, fmt.Errorf("winattr: security API unavailable")
	}

	path = normalizeWindowsPath(path)
	captureFileTimes(path, &entry)

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return entry, err
	}
	attrs, attrErr := windows.GetFileAttributes(pathPtr)
	if attrErr == nil {
		entry.Attributes = attrs
	}

	sddl, sddlErr := readSDDL(pathPtr)
	if sddlErr != nil {
		if entry.HasMeta() {
			return entry, nil
		}
		return entry, sddlErr
	}
	entry.SDDL = sddl
	return entry, nil
}

// Apply restores SDDL and file attributes. Requires sufficient privileges.
func Apply(path string, e Entry) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("winattr apply panic: %v", r)
		}
	}()
	if !e.HasMeta() {
		return nil
	}

	path = normalizeWindowsPath(path)
	needSecurity := e.SDDL != "" || e.Attributes != 0
	if needSecurity {
		if !securityAPIAvailable {
			return fmt.Errorf("winattr: security API unavailable")
		}
		if e.SDDL != "" {
			if err := writeSDDL(path, e.SDDL); err != nil {
				return err
			}
		}
		if e.Attributes != 0 {
			pathPtr, err := windows.UTF16PtrFromString(path)
			if err != nil {
				return err
			}
			if err := windows.SetFileAttributes(pathPtr, e.Attributes); err != nil {
				return fmt.Errorf("SetFileAttributes %q: %w", path, err)
			}
		}
	}
	if err := applyFileTimes(path, e); err != nil {
		return fmt.Errorf("SetFileTime %q: %w", path, err)
	}
	return nil
}

// ACLHash returns sha256 hex of security descriptor + attributes for incremental detection.
// Timestamps are excluded — atime changes on every read and would cause false positives.
func ACLHash(e Entry) string {
	if e.SDDL == "" && e.Attributes == 0 {
		return ""
	}
	payload := e.SDDL + "|" + strconv.FormatUint(uint64(e.Attributes), 10)
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}

func normalizeWindowsPath(path string) string {
	if strings.HasPrefix(path, `\\?\`) {
		return path
	}
	if len(path) >= 260 {
		if strings.HasPrefix(path, `\\`) {
			return `\\?\UNC\` + strings.TrimPrefix(path, `\\`)
		}
		return `\\?\` + path
	}
	return path
}

func readSDDL(pathPtr *uint16) (string, error) {
	var pSD uintptr
	r, _, _ := procGetNamedSecurityInfoW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(seFileObject),
		uintptr(securityInformation),
		0, 0, 0, 0,
		uintptr(unsafe.Pointer(&pSD)),
	)
	if r != 0 {
		return "", fmt.Errorf("GetNamedSecurityInfo: %w", syscall.Errno(r))
	}
	if pSD != 0 {
		defer procLocalFree.Call(pSD)
	}

	var strPtr uintptr
	var strLen uint32
	r, _, _ = procConvertSDToStringSDW.Call(
		pSD,
		1,
		uintptr(securityInformation),
		uintptr(unsafe.Pointer(&strPtr)),
		uintptr(unsafe.Pointer(&strLen)),
	)
	if r == 0 {
		return "", fmt.Errorf("ConvertSecurityDescriptorToStringSecurityDescriptor failed")
	}
	if strPtr == 0 {
		return "", nil
	}
	defer procLocalFree.Call(strPtr)

	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(strPtr))), nil
}

func writeSDDL(path, sddl string) error {
	sddlPtr, err := windows.UTF16PtrFromString(sddl)
	if err != nil {
		return err
	}
	var pSD uintptr
	r, _, _ := procConvertStringSDToSDW.Call(
		uintptr(unsafe.Pointer(sddlPtr)),
		1,
		uintptr(unsafe.Pointer(&pSD)),
		0,
	)
	if r == 0 {
		return fmt.Errorf("ConvertStringSecurityDescriptorToSecurityDescriptor failed")
	}
	if pSD != 0 {
		defer procLocalFree.Call(pSD)
	}

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	r, _, _ = procSetFileSecurityW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(securityInformation),
		pSD,
	)
	if r == 0 {
		return fmt.Errorf("SetFileSecurity %q failed", path)
	}
	return nil
}
