package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"pbs-win-backup/internal/branding"
	"pbs-win-backup/internal/backup"
	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/credential"
	"pbs-win-backup/internal/destination"
	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/history"
	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/pbsbackup"
	"pbs-win-backup/internal/restore"
	"pbs-win-backup/internal/status"
	"pbs-win-backup/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "status":
		cmdStatus()
	case "test":
		cmdTest()
	case "backup":
		cmdBackup()
	case "restore":
		cmdRestore()
	case "version":
		fmt.Println(branding.CLIName, version.Version)
		fmt.Println(branding.Copyright)
		fmt.Println(branding.DistributionNotice)
		fmt.Println("Author:", branding.AuthorName)
		fmt.Println("Telegram:", branding.TelegramHandle, branding.TelegramURL)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`%s CLI %s

  %s status
  %s test --server-id <id>
  %s backup --job-id <id> [--force-full]
  %s restore --job-id <id> --file <path> [--snapshot latest] [--dest <path>]
  %s restore --job-id <id> --folder <path> [--snapshot latest] [--dest <path>]
  %s version
`, branding.Name, version.Version, branding.CLIName, branding.CLIName, branding.CLIName, branding.CLIName, branding.CLIName, branding.CLIName)
	fmt.Println(branding.Copyright)
	fmt.Println(branding.DistributionNotice)
	fmt.Printf("Author: %s\n", branding.AuthorName)
	fmt.Printf("Telegram: %s %s\n", branding.TelegramHandle, branding.TelegramURL)
}

func cmdStatus() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка конфига: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Назначений: %d\n", len(cfg.Destinations))
	fmt.Printf("Заданий: %d\n", len(cfg.Jobs))
	fmt.Printf("Hostname: %s\n", backup.Hostname())
	fmt.Printf("Author: %s\n", branding.AuthorName)
	fmt.Printf("%s\n", branding.Copyright)
	fmt.Printf("Telegram: %s %s\n", branding.TelegramHandle, branding.TelegramURL)
}

func cmdTest() {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	serverID := fs.String("server-id", "", "ID сервера PBS")
	_ = fs.Parse(os.Args[2:])

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
		os.Exit(1)
	}

	dest, ok := models.FindDestination(cfg, *serverID)
	if !ok {
		fmt.Fprintf(os.Stderr, "назначение не найдено\n")
		os.Exit(1)
	}

	secret, err := credential.GetSecret(dest.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "secret не найден: %v\n", err)
		os.Exit(1)
	}

	result := destination.Test(*dest, secret)
	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	if !result.OK {
		os.Exit(2)
	}
}

func cmdBackup() {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	jobID := fs.String("job-id", "", "ID задания")
	forceFull := fs.Bool("force-full", false, "полный бэкап (очистить локальный индекс chunks)")
	_ = fs.Parse(os.Args[2:])

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ошибка: %v\n", err)
		os.Exit(1)
	}
	pbsbackup.SetChunkWorkersSetting(cfg.Settings.ChunkWorkers)

	var job *models.BackupJob
	for i := range cfg.Jobs {
		if cfg.Jobs[i].ID == *jobID {
			job = &cfg.Jobs[i]
			break
		}
	}
	if job == nil {
		fmt.Fprintf(os.Stderr, "задание не найдено\n")
		os.Exit(1)
	}
	dest, ok := models.FindDestination(cfg, job.EffectiveDestinationID())
	if !ok {
		fmt.Fprintf(os.Stderr, "назначение не найдено\n")
		os.Exit(1)
	}
	secret, err := credential.GetSecret(dest.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "secret: %v\n", err)
		os.Exit(1)
	}

	engine := backup.NewEngine(func(ev models.ProgressEvent) {
		fmt.Printf("[%s] %.1f%% %s\n", ev.Phase, ev.Percent, ev.Message)
	})

	result, err := engine.Run(context.Background(), backup.RunParams{
		Job:               *job,
		Destination:       dest,
		Secret:            secret,
		GlobalExclusions:  cfg.Settings.DefaultExclusions,
		ForceFull:         *forceFull,
		BandwidthMbps:     cfg.Settings.BandwidthMbps,
		NetworkTimeoutSec: cfg.Settings.NetworkTimeoutSec,
	})
	_ = history.Append(result)
	if err := status.WriteLastStatus(status.FromJobResult(result, backup.Hostname())); err != nil {
		eventlog.Error("CLI: не удалось записать last_status: " + err.Error())
	}

	if err != nil {
		eventlog.Error("CLI backup: " + err.Error())
		fmt.Fprintf(os.Stderr, "ошибка бэкапа: %v\n", err)
		os.Exit(1)
	}
	eventlog.Info(fmt.Sprintf("CLI backup OK: %s", job.Name))
	fmt.Printf("OK %s transferred=%d reused=%d\n", result.BackupType, result.BytesTransferred, result.BytesReused)
}

func cmdRestore() {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	jobID := fs.String("job-id", "", "ID задания")
	filePath := fs.String("file", "", "путь файла в снапшоте")
	folderPath := fs.String("folder", "", "путь папки в снапшоте")
	snapshot := fs.String("snapshot", "latest", "время снапшота")
	dest := fs.String("dest", "", "куда восстановить")
	_ = fs.Parse(os.Args[2:])

	cfg, _ := config.Load()
	var job models.BackupJob
	for _, j := range cfg.Jobs {
		if j.ID == *jobID {
			job = j
			break
		}
	}
	svc := restore.New(func(id string) (*models.PBSServer, error) {
		dest, ok := models.FindDestination(cfg, id)
		if !ok || !dest.IsPBS() {
			return nil, i18nconfig.FromConfig().E("cli.pbs_server_not_found")
		}
		s := dest.ToPBSServer()
		return &s, nil
	})

	if *folderPath != "" {
		req := models.RestoreFolderRequest{
			JobID:      job.ID,
			Snapshot:   *snapshot,
			FolderPath: *folderPath,
			DestPath:   *dest,
			ToOriginal: *dest == "",
		}
		r := svc.RestoreFolder(context.Background(), req, job, nil)
		if !r.OK {
			fmt.Fprintf(os.Stderr, "%s\n", r.Message)
			os.Exit(1)
		}
		fmt.Printf("%s (%d files)\n", r.Message, r.Count)
		return
	}

	req := models.RestoreRequest{
		JobID:      job.ID,
		Snapshot:   *snapshot,
		FilePath:   *filePath,
		DestPath:   *dest,
		ToOriginal: *dest == "",
	}
	r := svc.Restore(context.Background(), req, job, nil)
	if !r.OK {
		fmt.Fprintf(os.Stderr, "%s\n", r.Message)
		os.Exit(1)
	}
	fmt.Println(r.Path)
}
