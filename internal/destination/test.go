package destination

import (
	"pbs-win-backup/internal/ftpclient"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/pbs"
	"pbs-win-backup/internal/pbsbackup"
	"pbs-win-backup/internal/smbclient"
)

var (
	smbConnectTest = smbclient.Test
	ftpConnectTest = ftpclient.Test
	smbWriteTest   = smbclient.TestWriteAccess
	ftpWriteTest   = ftpclient.TestWriteAccess
)

func Test(dest models.BackupDestination, secret string) models.ConnectionTestResult {
	return TestLang(dest, secret, i18nconfig.FromConfig().Lang())
}

func TestLang(dest models.BackupDestination, secret, lang string) models.ConnectionTestResult {
	b := i18n.New(lang)
	switch dest.NormalizedType() {
	case models.DestPBS:
		if secret == "" {
			return models.ConnectionTestResult{OK: false, Message: b.T("dest.need_secret")}
		}
		return pbs.NewClient(dest.ToPBSServer(), secret).TestConnection()
	case models.DestSMB:
		if secret == "" {
			return models.ConnectionTestResult{OK: false, Message: b.T("dest.need_password")}
		}
		if err := smbConnectTest(dest, secret); err != nil {
			return models.ConnectionTestResult{OK: false, Message: err.Error()}
		}
		return models.ConnectionTestResult{OK: true, Message: b.T("dest.smb_ok")}
	case models.DestFTP:
		if secret == "" && dest.Username != "" && dest.Username != "anonymous" {
			return models.ConnectionTestResult{OK: false, Message: b.T("dest.need_password")}
		}
		if err := ftpConnectTest(dest, secret); err != nil {
			return models.ConnectionTestResult{OK: false, Message: err.Error()}
		}
		return models.ConnectionTestResult{OK: true, Message: b.T("dest.ftp_ok")}
	default:
		return models.ConnectionTestResult{OK: false, Message: b.Tf("dest.unknown_type", map[string]string{"type": dest.Type})}
	}
}

func TestFull(dest models.BackupDestination, secret, backupID string) models.ConnectionTestResult {
	return TestFullLang(dest, secret, backupID, i18nconfig.FromConfig().Lang())
}

func TestFullLang(dest models.BackupDestination, secret, backupID, lang string) models.ConnectionTestResult {
	b := i18n.New(lang)
	result := TestLang(dest, secret, lang)
	if !result.OK {
		return result
	}
	switch dest.NormalizedType() {
	case models.DestPBS:
		if err := pbsbackup.ProbeBackupAccess(dest.ToPBSServer(), secret, backupID); err != nil {
			return models.ConnectionTestResult{
				OK:         false,
				Message:    b.Tf("dest.protocol_failed", map[string]string{"message": result.Message, "err": err.Error()}),
				PBSVersion: result.PBSVersion,
			}
		}
	case models.DestSMB:
		if err := smbWriteTest(dest, secret, backupID); err != nil {
			return models.ConnectionTestResult{
				OK:      false,
				Message: b.Tf("dest.protocol_failed", map[string]string{"message": result.Message, "err": err.Error()}),
			}
		}
	case models.DestFTP:
		if err := ftpWriteTest(dest, secret, backupID); err != nil {
			return models.ConnectionTestResult{
				OK:      false,
				Message: b.Tf("dest.protocol_failed", map[string]string{"message": result.Message, "err": err.Error()}),
			}
		}
	}
	return result
}
