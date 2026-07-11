//go:build windows
// +build windows

package snapshot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/st-matskevich/go-vss"
)

func SymlinkSnapshot(symlinkPath string, id string, deviceObjectPath string) (string, error) {

	snapshotSymLinkFolder := symlinkPath + "\\" + id + "\\"

	snapshotSymLinkFolder = filepath.Clean(snapshotSymLinkFolder)
	os.RemoveAll(snapshotSymLinkFolder)
	if err := os.MkdirAll(snapshotSymLinkFolder, 0700); err != nil {
		return "", fmt.Errorf("failed to create snapshot symlink folder for snapshot: %s, err: %s", id, err)
	}

	os.Remove(snapshotSymLinkFolder)

	fmt.Println("Symlink from: ", deviceObjectPath, " to: ", snapshotSymLinkFolder)

	if err := os.Symlink(deviceObjectPath, snapshotSymLinkFolder); err != nil {
		return "", fmt.Errorf("failed to create symlink from: %s to: %s, error: %s", deviceObjectPath, snapshotSymLinkFolder, err)
	}

	return snapshotSymLinkFolder, nil
}

func getAppDataFolder() (string, error) {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	appDataFolder := filepath.Join(base, "ClickRAX", "vss")
	if err := os.MkdirAll(appDataFolder, 0o700); err != nil {
		return "", err
	}
	return appDataFolder, nil
}

func CreateVSSSnapshot(paths []string, backup_callback func(sn map[string]SnapShot) error) error {
	sn := vss.Snapshotter{}
	defer sn.Release()
	snapshots := make(map[string]SnapShot)

	for _, path := range paths {
		path, _ = filepath.Abs(path)
		volName := filepath.VolumeName(path)
		volName += "\\"
		subPath := path[len(volName):]

		appDataFolder, err := getAppDataFolder()
		if err != nil {
			return err
		}

		snapshot, err := sn.CreateSnapshot(volName, false, 180)
		if err != nil {
			return err
		}

		_, err = SymlinkSnapshot(filepath.Join(appDataFolder, "VSS"), snapshot.Id, snapshot.DeviceObjectPath)
		if err != nil {
			return err
		}

		snapshots[path] = SnapShot{FullPath: filepath.Join(appDataFolder, "VSS", snapshot.Id, subPath), Id: snapshot.Id, ObjectPath: snapshot.DeviceObjectPath, Valid: true}
	}

	return backup_callback(snapshots)
}

func VSSCleanup() {

}
