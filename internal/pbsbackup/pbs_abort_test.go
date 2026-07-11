package pbsbackup

import (
	"testing"

	pbscommon "pbscommon"
)

func TestAbortBackupSessionNilSafe(t *testing.T) {
	var client pbscommon.PBSClient
	client.AbortBackupSession()
}
