//go:build windows

package service

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"pbs-win-backup/internal/branding"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/paths"
	"pbs-win-backup/internal/winutil"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const displayName = branding.Title

func loc() *i18n.Bundle {
	return i18nconfig.FromConfig()
}

func openManager() (*mgr.Mgr, error) {
	return mgr.Connect()
}

func openService(m *mgr.Mgr) (*mgr.Service, error) {
	return m.OpenService(serviceName)
}

func syncServiceBinary() (string, error) {
	target, err := paths.ServiceBinaryPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", loc().Ef("service.install.dir_create", map[string]string{"err": err.Error()})
	}

	source, err := os.Executable()
	if err != nil {
		return "", err
	}
	source, _ = filepath.Abs(source)
	target, _ = filepath.Abs(target)

	if strings.EqualFold(source, target) {
		return target, nil
	}

	if err := copyExecutable(source, target); err != nil {
		return "", loc().Ef("service.install.copy", map[string]string{"path": target, "err": err.Error()})
	}
	return target, nil
}

func copyExecutable(src, dst string) error {
	tmp := dst + ".new"
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func isFileInUse(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "being used by another process") ||
		strings.Contains(msg, "used by another process") ||
		strings.Contains(msg, "занят другим процессом")
}

func stopRunningService() error {
	m, err := openManager()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := openExistingService(m)
	if err != nil {
		if isServiceNotExist(err) {
			return nil
		}
		return err
	}
	defer s.Close()

	st, err := s.Query()
	if err != nil {
		return err
	}
	if st.State == svc.Stopped {
		return nil
	}
	return stopServiceWait(s)
}

func upgradeServiceBinary() (string, error) {
	if err := stopRunningService(); err != nil {
		return "", loc().Ef("service.install.stop_for_upgrade", map[string]string{"err": err.Error()})
	}
	var lastErr error
	for attempt := 0; attempt < 25; attempt++ {
		if attempt > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		target, err := syncServiceBinary()
		if err == nil {
			return target, nil
		}
		lastErr = err
		if !isFileInUse(err) {
			return "", err
		}
	}
	targetPath, _ := paths.ServiceBinaryPath()
	return "", loc().Ef("service.install.copy_retry", map[string]string{
		"path": targetPath,
		"err":  lastErr.Error(),
		"app":  branding.Name,
	})
}

func serviceCommandLine(exe string) string {
	return fmt.Sprintf(`"%s" --service`, exe)
}

func normalizeServicePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, `"`)
	if idx := strings.Index(strings.ToLower(p), ".exe"); idx >= 0 {
		p = p[:idx+4]
	}
	return strings.ToLower(filepath.Clean(p))
}

func registerEventSource() error {
	err := eventlog.InstallAsEventCreate(serviceName, eventlog.Info|eventlog.Warning|eventlog.Error)
	if err == nil || isEventLogSourceExists(err) {
		return nil
	}
	return err
}

func isEventLogSourceExists(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") || strings.Contains(msg, "registry key already exists")
}

func isMarkedForDeletion(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, windows.ERROR_SERVICE_MARKED_FOR_DELETE) {
		return true
	}
	if errno, ok := err.(syscall.Errno); ok && errno == windows.ERROR_SERVICE_MARKED_FOR_DELETE {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "marked for deletion")
}

func isServiceNotExist(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return true
	}
	if errno, ok := err.(syscall.Errno); ok && errno == windows.ERROR_SERVICE_DOES_NOT_EXIST {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "не существует")
}

func waitServiceDeleted(m *mgr.Mgr, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s, err := m.OpenService(serviceName)
		if err != nil {
			if isMarkedForDeletion(err) || isServiceNotExist(err) {
				if isServiceNotExist(err) {
					return nil
				}
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return err
		}
		_ = s.Close()
		time.Sleep(500 * time.Millisecond)
	}
	return loc().Ef("service.install.still_deleting", nil)
}

func forceDeleteServiceSC() error {
	_ = winutil.HiddenCommand("sc.exe", "stop", serviceName).Run()
	time.Sleep(2 * time.Second)
	out, err := winutil.HiddenCommand("sc.exe", "delete", serviceName).CombinedOutput()
	msg := strings.TrimSpace(string(out))
	if err == nil {
		time.Sleep(time.Second)
		return nil
	}
	lower := strings.ToLower(msg + " " + err.Error())
	if strings.Contains(lower, "1060") || strings.Contains(lower, "does not exist") || strings.Contains(lower, "не существует") {
		return nil
	}
	if strings.Contains(lower, "1072") || strings.Contains(lower, "marked for deletion") {
		return nil
	}
	if msg != "" {
		return fmt.Errorf("sc delete: %s", msg)
	}
	return err
}

func openExistingService(m *mgr.Mgr) (*mgr.Service, error) {
	return m.OpenService(serviceName)
}

func createServiceWithRetry(m *mgr.Mgr, target string) (*mgr.Service, error) {
	var lastErr error
	for attempt := 0; attempt < 30; attempt++ {
		s, err := m.CreateService(serviceName, target, mgr.Config{
			DisplayName: displayName,
			Description: loc().T("service.install.description"),
			StartType:   mgr.StartAutomatic,
		}, "--service")
		if err == nil {
			return s, nil
		}
		lastErr = err
		if isMarkedForDeletion(err) {
			time.Sleep(time.Second)
			continue
		}
		return nil, err
	}
	return nil, loc().Ef("service.install.create_retry", map[string]string{"err": lastErr.Error()})
}

func ensureServiceCommandLine(s *mgr.Service, target string) error {
	cfg, err := s.Config()
	if err != nil {
		return err
	}
	want := serviceCommandLine(target)
	got := strings.TrimSpace(cfg.BinaryPathName)
	if normalizeServicePath(got) == normalizeServicePath(want) {
		return nil
	}
	if err := stopServiceWait(s); err != nil {
		return err
	}
	// Preserve ServiceType, ErrorControl, ServiceStartName, etc. — zero values cause ERROR_INVALID_PARAMETER.
	cfg.BinaryPathName = want
	cfg.DisplayName = displayName
	cfg.Description = loc().T("service.install.description")
	cfg.StartType = mgr.StartAutomatic
	return s.UpdateConfig(cfg)
}

func stopServiceWait(s *mgr.Service) error {
	st, err := s.Query()
	if err != nil {
		return err
	}
	if st.State == svc.Stopped {
		return nil
	}
	if _, err := s.Control(svc.Stop); err != nil {
		return err
	}
	for i := 0; i < 30; i++ {
		st, err = s.Query()
		if err != nil {
			return err
		}
		if st.State == svc.Stopped {
			return nil
		}
		time.Sleep(time.Second)
	}
	return loc().Ef("service.install.stop_timeout", nil)
}

func waitUntilRunning(s *mgr.Service) error {
	for i := 0; i < 40; i++ {
		st, err := s.Query()
		if err != nil {
			return err
		}
		switch st.State {
		case svc.Running:
			return nil
		case svc.Stopped:
			return describeStartFailure(st, s)
		case svc.StartPending:
			time.Sleep(500 * time.Millisecond)
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
	return loc().Ef("service.install.start_timeout", nil)
}

func describeStartFailure(st svc.Status, s *mgr.Service) error {
	cfg, _ := s.Config()
	path := cfg.BinaryPathName
	switch st.Win32ExitCode {
	case 5:
		return loc().Ef("service.install.access_denied", map[string]string{"path": path})
	case 1064:
		return loc().Ef("service.install.interactive", map[string]string{"path": path})
	default:
		if st.Win32ExitCode != 0 && st.Win32ExitCode != 1077 {
			return loc().Ef("service.install.stopped_code", map[string]string{
				"code": fmt.Sprintf("%d", st.Win32ExitCode),
				"path": path,
			})
		}
	}
	return loc().Ef("service.install.not_started", map[string]string{"path": path})
}

func Install() error {
	target, err := upgradeServiceBinary()
	if err != nil {
		return err
	}
	if err := registerEventSource(); err != nil {
		return loc().Ef("service.install.eventlog", map[string]string{"err": err.Error()})
	}

	m, err := openManager()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	for attempt := 0; attempt < 3; attempt++ {
		err := installOnce(m, target)
		if err == nil {
			return nil
		}
		if isMarkedForDeletion(err) {
			if waitErr := waitServiceDeleted(m, 30*time.Second); waitErr != nil {
				return waitErr
			}
			continue
		}
		return err
	}
	return loc().Ef("service.install.failed", nil)
}

func installOnce(m *mgr.Mgr, target string) error {
	s, err := openExistingService(m)
	if err == nil {
		defer s.Close()
		st, qerr := s.Query()
		if qerr == nil && st.State != svc.Stopped {
			if err := stopServiceWait(s); err != nil {
				return err
			}
			time.Sleep(time.Second)
		}
		if err := ensureServiceCommandLine(s, target); err != nil {
			return loc().Ef("service.install.update_path", map[string]string{"err": err.Error()})
		}
		if err := s.Start(); err != nil {
			return loc().Ef("service.install.start", map[string]string{"err": err.Error()})
		}
		return waitUntilRunning(s)
	}
	if !isServiceNotExist(err) {
		return err
	}

	s, err = createServiceWithRetry(m, target)
	if err != nil {
		return loc().Ef("service.install.create", map[string]string{"err": err.Error()})
	}
	defer s.Close()

	if err := s.Start(); err != nil {
		return loc().Ef("service.install.start", map[string]string{"err": err.Error()})
	}
	return waitUntilRunning(s)
}

func Uninstall() error {
	m, err := openManager()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		if isServiceNotExist(err) {
			return nil
		}
		if isMarkedForDeletion(err) {
			_ = waitServiceDeleted(m, 60*time.Second)
			return nil
		}
		return forceDeleteServiceSC()
	}

	_ = stopServiceWait(s)
	if delErr := s.Delete(); delErr != nil && !isMarkedForDeletion(delErr) {
		_ = s.Close()
		if scErr := forceDeleteServiceSC(); scErr != nil {
			return scErr
		}
		_ = waitServiceDeleted(m, 30*time.Second)
		return nil
	}
	_ = s.Close()

	if waitErr := waitServiceDeleted(m, 60*time.Second); waitErr != nil {
		_ = forceDeleteServiceSC()
	}
	return nil
}

func Start() error {
	target, err := upgradeServiceBinary()
	if err != nil {
		return err
	}

	m, err := openManager()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := openExistingService(m)
	if err != nil {
		if isServiceNotExist(err) {
			return loc().Ef("service.install.not_installed", nil)
		}
		return err
	}
	defer s.Close()

	if err := ensureServiceCommandLine(s, target); err != nil {
		return loc().Ef("service.install.update_path", map[string]string{"err": err.Error()})
	}
	if err := s.Start(); err != nil {
		return loc().Ef("service.install.start", map[string]string{"err": err.Error()})
	}
	return waitUntilRunning(s)
}

func Stop() error {
	m, err := openManager()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := openExistingService(m)
	if err != nil {
		if isServiceNotExist(err) {
			return loc().Ef("service.install.not_installed", nil)
		}
		return err
	}
	defer s.Close()
	return stopServiceWait(s)
}

func Restart() error {
	target, err := upgradeServiceBinary()
	if err != nil {
		return err
	}

	m, err := openManager()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := openExistingService(m)
	if err != nil {
		if isServiceNotExist(err) {
			return loc().Ef("service.install.not_installed", nil)
		}
		return err
	}
	defer s.Close()

	if err := ensureServiceCommandLine(s, target); err != nil {
		return err
	}
	if err := stopServiceWait(s); err != nil {
		return err
	}
	if err := s.Start(); err != nil {
		return loc().Ef("service.install.start", map[string]string{"err": err.Error()})
	}
	return waitUntilRunning(s)
}
