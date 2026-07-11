//go:build windows

package health

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"

	"pbs-win-backup/internal/credential"
	"pbs-win-backup/internal/destination"
	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/models"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpace = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func diskFreeGB(path string) (float64, error) {
	if path == "" {
		return 0, i18nconfig.FromConfig().E("health.empty_path")
	}
	vol := filepath.VolumeName(path)
	if vol == "" {
		vol = path
	}
	p, err := syscall.UTF16PtrFromString(vol + `\`)
	if err != nil {
		return 0, err
	}
	var free, total, avail uint64
	r, _, e := getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&avail)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&free)),
	)
	if r == 0 {
		return 0, e
	}
	return float64(avail) / (1024 * 1024 * 1024), nil
}

func Run(cfg *models.Config) Report {
	b := i18nconfig.FromConfig()
	checks := make([]Check, 0)

	for _, job := range cfg.Jobs {
		for _, src := range job.Sources {
			gb, err := diskFreeGB(src)
			if err != nil {
				checks = append(checks, Check{
					Name: "disk:" + src, OK: false, Message: err.Error(),
				})
				continue
			}
			ok := gb > 1.0
			msg := b.Tf("health.disk_free", map[string]string{"n": fmt.Sprintf("%.1f", gb)})
			if !ok {
				msg = b.Tf("health.disk_low", map[string]string{"msg": msg})
			}
			checks = append(checks, Check{Name: "disk:" + src, OK: ok, Message: msg})
		}
	}

	for _, d := range cfg.Destinations {
		secret, err := credential.GetSecret(d.ID)
		if err != nil {
			checks = append(checks, Check{
				Name: destLabel(d), OK: false, Message: b.T("health.cred_not_found"),
			})
			continue
		}
		r := destination.Test(d, secret)
		checks = append(checks, Check{
			Name: destLabel(d), OK: r.OK, Message: r.Message,
		})
	}

	allOK := true
	for _, c := range checks {
		if !c.OK {
			allOK = false
			break
		}
	}
	return Report{Checks: checks, OK: allOK}
}

func destLabel(d models.BackupDestination) string {
	return d.NormalizedType() + ":" + d.Name
}
