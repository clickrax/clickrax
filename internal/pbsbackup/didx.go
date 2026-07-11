package pbsbackup

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"time"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/pbs"
	"pbscommon"
)

var didxMagicBytes = []byte{28, 145, 78, 165, 25, 186, 179, 205}
var catalogMagicBytes = []byte{145, 253, 96, 249, 196, 103, 88, 213}

type didxRecord struct {
	offset uint64
	digest string
}

func parseDidxRecords(didx []byte) ([]didxRecord, error) {
	if len(didx) < 4096 {
		if len(didx) > 0 && didx[0] == '{' {
			return nil, i18n.Ef("pbs.didx.pbs_response", map[string]string{"msg": strings.TrimSpace(string(didx))})
		}
		return nil, i18n.Ef("pbs.didx.too_short", map[string]string{"n": fmt.Sprintf("%d", len(didx))})
	}
	if !bytes.HasPrefix(didx, didxMagicBytes) {
		return nil, i18n.E("pbs.didx.bad_magic", nil)
	}
	body := didx[4096:]
	records := make([]didxRecord, 0, len(body)/40)
	for i := 0; i+40 <= len(body); i += 40 {
		off := binary.LittleEndian.Uint64(body[i : i+8])
		dig := fmt.Sprintf("%x", body[i+8:i+40])
		records = append(records, didxRecord{offset: off, digest: dig})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].offset < records[j].offset })
	return records, nil
}

func reassembleDynamicStream(client *pbscommon.PBSClient, archiveName string) ([]byte, error) {
	raw, err := client.DownloadToBytes(archiveName)
	if err != nil {
		return nil, i18n.Ef("pbs.didx.load", map[string]string{"name": archiveName, "err": err.Error()})
	}
	return reassembleDynamicBytes(raw, client.GetChunkData)
}

func reassembleDynamicBytes(raw []byte, getChunk func(digest string) ([]byte, error)) ([]byte, error) {
	if len(raw) == 0 {
		return nil, i18n.E("pbs.didx.empty_pbs", nil)
	}
	if bytes.HasPrefix(raw, catalogMagicBytes) {
		return raw, nil
	}
	records, err := parseDidxRecords(raw)
	if err != nil {
		return nil, err
	}
	return reassembleFromRecords(records, getChunk)
}

// reassembleFromRecords joins chunks; didx offsets are chunk END positions (PBS spec).
func reassembleFromRecords(records []didxRecord, getChunk func(digest string) ([]byte, error)) ([]byte, error) {
	if len(records) == 0 {
		return nil, i18n.E("pbs.didx.no_chunks", nil)
	}
	total := records[len(records)-1].offset
	if total == 0 {
		return nil, i18n.E("pbs.didx.zero_stream", nil)
	}
	out := make([]byte, total)
	var start uint64
	for _, r := range records {
		chunk, err := getChunk(r.digest)
		if err != nil {
			return nil, fmt.Errorf("chunk %s: %w", r.digest[:min(12, len(r.digest))], err)
		}
		end := start + uint64(len(chunk))
		if end != r.offset {
			return nil, i18n.Ef("pbs.didx.offset_mismatch", map[string]string{
				"end":    fmt.Sprintf("%d", end),
				"offset": fmt.Sprintf("%d", r.offset),
			})
		}
		copy(out[start:], chunk)
		start = end
	}
	return out, nil
}

// CatalogAvailable checks catalog.pcat1.didx via the backup reader API (same path as restore).
func CatalogAvailable(server models.PBSServer, secret string, ref SnapshotRef) (bool, error) {
	return archiveAvailable(server, secret, ref, "catalog.pcat1.didx", catalogMagicBytes)
}

// PxarAvailable checks that a pxar data archive exists in the snapshot.
func PxarAvailable(server models.PBSServer, secret string, ref SnapshotRef) (bool, error) {
	_, err := resolvePxarArchive(server, secret, ref)
	if err != nil {
		if strings.Contains(err.Error(), "отсутствует") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func archiveAvailable(server models.PBSServer, secret string, ref SnapshotRef, archiveName string, magic []byte) (bool, error) {
	raw, err := readerDownloadIndex(server, secret, ref, archiveName)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "404") || strings.Contains(msg, "no such file") || strings.Contains(msg, "not found") || strings.Contains(msg, "unable to open") {
			return false, nil
		}
		return false, err
	}
	if len(raw) < 8 {
		return false, nil
	}
	if bytes.HasPrefix(raw, magic) || bytes.HasPrefix(raw, didxMagicBytes) {
		return true, nil
	}
	return false, nil
}

func resolvePxarArchive(server models.PBSServer, secret string, ref SnapshotRef) (string, error) {
	if ref.BackupID == "" {
		return "", i18n.E("pbs.didx.backup_id_missing", nil)
	}
	unix := ref.BackupTime
	if unix == 0 {
		var err error
		unix, err = snapshotToUnix(ref.Time)
		if err != nil {
			return "", err
		}
	}

	files, err := pbs.NewClient(server, secret).SnapshotManifestFiles("host", ref.BackupID, unix)
	if err == nil {
		for _, f := range files {
			if strings.HasSuffix(f, ".pxar.didx") {
				return f, nil
			}
		}
	}

	for _, name := range []string{"backup.pxar.didx"} {
		ok, err := archiveAvailable(server, secret, ref, name, didxMagicBytes)
		if err != nil {
			return "", err
		}
		if ok {
			return name, nil
		}
	}
	return "", i18n.E("pbs.didx_missing", nil)
}

func openReader(server models.PBSServer, secret string, ref SnapshotRef) (*pbscommon.PBSClient, func(), error) {
	client, err := connectReader(server, secret, ref)
	if err != nil {
		return nil, nil, err
	}
	return client, func() { closeReader(client) }, nil
}

func snapshotToUnix(snapshotTime string) (int64, error) {
	if snapshotTime == "" || snapshotTime == "latest" {
		return 0, i18n.E("pbs.didx.snapshot_time_missing", nil)
	}
	t, err := time.Parse(time.RFC3339, snapshotTime)
	if err != nil {
		var u int64
		if _, scanErr := fmt.Sscanf(snapshotTime, "%d", &u); scanErr != nil {
			return 0, i18n.Ef("pbs.didx.snapshot_time_invalid", map[string]string{"time": snapshotTime})
		}
		return u, nil
	}
	return t.Unix(), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
