package models

import "time"

type PBSServer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Fingerprint string `json:"fingerprint"`
	Datastore   string `json:"datastore"`
	Namespace   string `json:"namespace"`
	TokenID     string `json:"token_id"`
	Description string `json:"description,omitempty"`
}

type Schedule struct {
	Enabled           bool     `json:"enabled"`
	Type              string   `json:"type"` // daily, weekly, startup
	Time              string   `json:"time,omitempty"`  // legacy: first time HH:MM
	Times             []string `json:"times,omitempty"` // one or more HH:MM per day
	Weekdays          []int    `json:"weekdays,omitempty"`
	RunOnStartup      bool     `json:"run_on_startup"`
	SkipIfRunning     bool     `json:"skip_if_running"`
	FullBackupMode    string   `json:"full_backup_mode,omitempty"`    // weekly, biweekly, monthly, never
	FullBackupWeekday int      `json:"full_backup_weekday,omitempty"` // 1=Mon … 7=Sun
	FullBackupAnchor  string   `json:"full_backup_anchor,omitempty"`  // YYYY-MM-DD, first full day for biweekly
}

type BackupJob struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	DestinationID      string   `json:"destination_id,omitempty"`
	ServerID           string   `json:"server_id,omitempty"` // legacy alias for destination_id
	SourceMode         string   `json:"source_mode,omitempty"` // volume, paths
	Sources            []string `json:"sources"`
	Exclusions         []string `json:"exclusions,omitempty"`
	BackupID           string   `json:"backup_id"`
	VSSEnabled         bool     `json:"vss_enabled"`
	SplitEnabled       bool     `json:"split_enabled"`
	SplitSizeGB        int      `json:"split_size_gb"`
	SkipAccessErrors   bool     `json:"skip_access_errors"`
	LowPriorityIO      bool     `json:"low_priority_io"`
	EncryptionEnabled  bool     `json:"encryption_enabled"`
	Schedule           Schedule `json:"schedule"`
	VerifyAfterBackup  bool     `json:"verify_after_backup"`
	Comment            string   `json:"comment,omitempty"`
	NotifyBackup       string   `json:"notify_backup,omitempty"`  // inherit, off, always, failure
	NotifyRestore      string   `json:"notify_restore,omitempty"` // inherit, off, always, failure
}

type AppSettings struct {
	Language           string   `json:"language"`
	StartWithWindows   bool     `json:"start_with_windows"`
	MinimizeToTray     bool     `json:"minimize_to_tray"`
	DefaultServerID    string   `json:"default_server_id,omitempty"`
	DefaultExclusions  []string `json:"default_exclusions,omitempty"`
	BandwidthMbps      int      `json:"bandwidth_mbps"`
	ChunkWorkers       int      `json:"chunk_workers"` // 0 = auto (12–32 for 10G+)
	NetworkTimeoutSec  int      `json:"network_timeout_sec"`
	NetworkRetries     int      `json:"network_retries"`
	SkipBehavior       string   `json:"skip_behavior"` // continue, warning, error
	CriticalErrorLimit int      `json:"critical_error_limit"`
	RestoreOverwrite   string   `json:"restore_overwrite"` // ask, overwrite, backup
	LogLevel           string   `json:"log_level"`
	CheckUpdates       bool     `json:"check_updates"`
	WebhookURL         string       `json:"webhook_url,omitempty"`
	SMTP               SMTPSettings `json:"smtp,omitempty"`
	NotifyBackup       string       `json:"notify_backup,omitempty"`  // off, always, failure
	NotifyRestore      string       `json:"notify_restore,omitempty"` // off, always, failure
}

type SMTPSettings struct {
	Host        string `json:"host,omitempty"`
	Port        int    `json:"port,omitempty"`
	Username    string `json:"username,omitempty"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	InsecureTLS bool   `json:"insecure_tls,omitempty"`
}

type Config struct {
	Version      int                 `json:"version"`
	Destinations []BackupDestination `json:"destinations,omitempty"`
	Servers      []PBSServer         `json:"servers,omitempty"` // legacy, migrated to destinations
	Jobs         []BackupJob         `json:"jobs"`
	Settings     AppSettings         `json:"settings"`
}

type ServerStatus struct {
	ServerID    string `json:"server_id"`
	Online      bool   `json:"online"`
	Message     string `json:"message"`
	PBSVersion  string `json:"pbs_version,omitempty"`
	CheckedAt   string `json:"checked_at"`
}

type ConnectionTestResult struct {
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
	PBSVersion string `json:"pbs_version,omitempty"`
	Datastores []string `json:"datastores,omitempty"`
}

type ProgressPhase string

const (
	PhaseIdle       ProgressPhase = "idle"
	PhasePreparing  ProgressPhase = "preparing"
	PhaseAnalyzing  ProgressPhase = "analyzing"
	PhaseVSS        ProgressPhase = "vss"
	PhaseTransfer   ProgressPhase = "transfer"
	PhaseFinalizing ProgressPhase = "finalizing"
	PhaseVerify     ProgressPhase = "verify"
	PhaseDone       ProgressPhase = "done"
	PhaseCancelled  ProgressPhase = "cancelled"
	PhaseError      ProgressPhase = "error"
)

type ProgressEvent struct {
	JobID            string        `json:"job_id"`
	JobName          string        `json:"job_name"`
	Phase            ProgressPhase `json:"phase"`
	BackupType       string        `json:"backup_type,omitempty"`
	Percent          float64       `json:"percent"`
	BytesTransferred int64         `json:"bytes_transferred"`
	BytesReused      int64         `json:"bytes_reused"`
	BytesTotal       int64         `json:"bytes_total_estimate"`
	ChunksNew        int           `json:"chunks_new"`
	ChunksReused     int           `json:"chunks_reused"`
	ChunksTotal      int           `json:"chunks_total"`
	SpeedBps         int64         `json:"speed_bps"`
	ETASec           int64         `json:"eta_sec"`
	StartedAt        string        `json:"started_at,omitempty"`
	CurrentPath      string        `json:"current_path"`
	FilesDone        int           `json:"files_done"`
	FilesTotal       int           `json:"files_total"`
	FilesSkipped     int           `json:"files_skipped"`
	FilesChanged     int           `json:"files_changed"`
	Message          string        `json:"message"`
	Trigger          string        `json:"trigger,omitempty"`
}

type JobRunRecord struct {
	JobID            string `json:"job_id"`
	JobName          string `json:"job_name"`
	Status           string `json:"status"`
	BackupType       string `json:"backup_type"`
	Trigger          string `json:"trigger,omitempty"` // manual, scheduled
	StartedAt        string `json:"started_at"`
	FinishedAt       string `json:"finished_at"`
	DurationSec      int64  `json:"duration_sec"`
	BytesTransferred int64  `json:"bytes_transferred"`
	BytesReused      int64  `json:"bytes_reused"`
	FilesTotal       int    `json:"files_total"`
	FilesSkipped     int    `json:"files_skipped"`
	Snapshot         string `json:"snapshot,omitempty"`
	Error            string `json:"error,omitempty"`
	Message            string `json:"message,omitempty"`
}

type JobRunResult struct {
	JobID            string
	JobName          string
	Status           string
	BackupType       string
	Trigger          string
	StartedAt        time.Time
	FinishedAt       time.Time
	DurationSec      int64
	BytesTransferred int64
	BytesReused      int64
	FilesTotal       int
	FilesSkipped     int
	Snapshot         string
	Error            string
	Message          string
}

func (r JobRunResult) ToRecord() JobRunRecord {
	msg := r.Message
	if msg == "" && r.Error != "" {
		msg = r.Error
	}
	return JobRunRecord{
		JobID:            r.JobID,
		JobName:          r.JobName,
		Status:           r.Status,
		BackupType:       r.BackupType,
		Trigger:          r.Trigger,
		StartedAt:        r.StartedAt.Format(time.RFC3339),
		FinishedAt:       r.FinishedAt.Format(time.RFC3339),
		DurationSec:      r.DurationSec,
		BytesTransferred: r.BytesTransferred,
		BytesReused:      r.BytesReused,
		FilesTotal:       r.FilesTotal,
		FilesSkipped:     r.FilesSkipped,
		Snapshot:         r.Snapshot,
		Error:            r.Error,
		Message:          msg,
	}
}

type ExecutionRun struct {
	JobID            string  `json:"job_id"`
	JobName          string  `json:"job_name"`
	Trigger          string  `json:"trigger"`
	Phase            string  `json:"phase"`
	BackupType       string  `json:"backup_type,omitempty"`
	Percent          float64 `json:"percent"`
	BytesTransferred int64   `json:"bytes_transferred"`
	BytesReused      int64   `json:"bytes_reused"`
	SpeedBps         int64   `json:"speed_bps"`
	ETASec           int64   `json:"eta_sec"`
	ChunksNew        int     `json:"chunks_new"`
	ChunksReused     int     `json:"chunks_reused"`
	FilesDone        int     `json:"files_done"`
	FilesTotal       int     `json:"files_total"`
	FilesSkipped     int     `json:"files_skipped"`
	CurrentPath      string  `json:"current_path,omitempty"`
	Message          string  `json:"message"`
	StartedAt        string  `json:"started_at"`
	UpdatedAt        string  `json:"updated_at,omitempty"`
	CanStop          bool    `json:"can_stop"`
	CanRetry         bool    `json:"can_retry"`
	CanDismiss       bool    `json:"can_dismiss"`
}

type ScheduledRunInfo struct {
	JobID      string `json:"job_id"`
	JobName    string `json:"job_name"`
	RunAt      string `json:"run_at"`
	BackupType string `json:"backup_type"`
	TimesLabel string `json:"times_label"`
}

type QueuedRunInfo struct {
	JobID      string `json:"job_id"`
	JobName    string `json:"job_name"`
	Trigger    string `json:"trigger"`
	Position   int    `json:"position"`
	EnqueuedAt string `json:"enqueued_at"`
}

type ExecutionState struct {
	Active          []ExecutionRun     `json:"active"`
	Interrupted     []ExecutionRun     `json:"interrupted"`
	Queued          []QueuedRunInfo    `json:"queued"`
	Upcoming        []ScheduledRunInfo `json:"upcoming"`
	RecentManual    []JobRunRecord     `json:"recent_manual"`
	RecentScheduled []JobRunRecord     `json:"recent_scheduled"`
}

type LastStatus struct {
	Hostname         string `json:"hostname"`
	JobName          string `json:"job_name"`
	LastRun          string `json:"last_run"`
	LastSuccess      string `json:"last_success"`
	Status           string `json:"status"`
	BackupType       string `json:"backup_type"`
	DurationSec      int64  `json:"duration_sec"`
	BytesTransferred int64  `json:"bytes_transferred"`
	BytesReused      int64  `json:"bytes_reused"`
	Snapshot         string `json:"snapshot,omitempty"`
	Error            string `json:"error,omitempty"`
}

type SnapshotInfo struct {
	Time       string `json:"time"`
	Backup     string `json:"backup"`
	BackupTime int64  `json:"backup_time"`
	Comment    string `json:"comment,omitempty"`
	HasCatalog bool   `json:"has_catalog"`
}

type SnapshotFile struct {
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	IsDir      bool   `json:"is_dir"`
	Modified   string `json:"modified,omitempty"`
	Owner      string `json:"owner,omitempty"`
	Attributes string `json:"attributes,omitempty"`
}

type RestoreRequest struct {
	JobID      string `json:"job_id"`
	Snapshot   string `json:"snapshot"`
	FilePath   string `json:"file_path"`
	DestPath   string `json:"dest_path"`
	ToOriginal bool   `json:"to_original"`
	Overwrite  bool   `json:"overwrite"`
}

type RestoreResult struct {
	OK            bool   `json:"ok"`
	Message       string `json:"message"`
	Path          string `json:"path,omitempty"`
	NeedsConfirm  bool   `json:"needs_confirm"`
	ExistingPath  string `json:"existing_path,omitempty"`
}

type RestoreFolderRequest struct {
	JobID      string `json:"job_id"`
	Snapshot   string `json:"snapshot"`
	FolderPath string `json:"folder_path"`
	DestPath   string `json:"dest_path"`
	ToOriginal bool   `json:"to_original"`
	Overwrite  bool   `json:"overwrite"`
}

type RestoreFolderResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

type RestoreBatchRequest struct {
	JobID      string   `json:"job_id"`
	Snapshot   string   `json:"snapshot"`
	Paths      []string `json:"paths"`
	DestPath   string   `json:"dest_path"`
	ToOriginal bool     `json:"to_original"`
	Overwrite  bool     `json:"overwrite"`
}

type RestoreBatchResult struct {
	OK      bool     `json:"ok"`
	Message string   `json:"message"`
	Count   int      `json:"count"`
	Errors  []string `json:"errors,omitempty"`
}

type LastBackupInfo struct {
	JobID    string `json:"job_id"`
	JobName  string `json:"job_name"`
	Snapshot string `json:"snapshot,omitempty"`
	Status   string `json:"status"`
}

type VolumeFolder struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	System bool   `json:"system"`
}

type PathEstimate struct {
	Path    string `json:"path"`
	Files   int64  `json:"files"`
	Bytes   int64  `json:"bytes"`
	Approx  bool   `json:"approx,omitempty"`
	Volume  bool   `json:"volume,omitempty"`
	Error   string `json:"error,omitempty"`
}

type QuickBackupRequest struct {
	Name          string   `json:"name"`
	DestinationID string   `json:"destination_id,omitempty"`
	ServerID      string   `json:"server_id,omitempty"` // legacy
	Sources      []string `json:"sources"`
	VSSEnabled   bool     `json:"vss_enabled"`
	BackupID     string   `json:"backup_id"`
	SourceMode   string   `json:"source_mode,omitempty"`
	ForceFull    bool     `json:"force_full"`
	Exclusions   []string `json:"exclusions,omitempty"`
	Comment      string   `json:"comment,omitempty"`
}

type BackupCheckpoint struct {
	JobID        string `json:"job_id"`
	JobName      string `json:"job_name"`
	Phase        string `json:"phase"`
	NewChunks    uint64 `json:"new_chunks"`
	ReusedChunks uint64 `json:"reused_chunks"`
	Error        string `json:"error,omitempty"`
	UpdatedAt    string `json:"updated_at"`
}

type HealthCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type HealthReport struct {
	Checks []HealthCheck `json:"checks"`
	OK     bool          `json:"ok"`
}

type UpdateInfo struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	URL             string `json:"url,omitempty"`
	Message         string `json:"message"`
}

type ContactInfo struct {
	AuthorName         string `json:"author_name"`
	Copyright            string `json:"copyright"`
	DistributionNotice string `json:"distribution_notice"`
	TelegramUsername   string `json:"telegram_username"`
	TelegramHandle     string `json:"telegram_handle"`
	TelegramURL        string `json:"telegram_url"`
	GitHubURL          string `json:"github_url"`
}

type ServiceStatus struct {
	Installed     bool   `json:"installed"`
	Running       bool   `json:"running"`
	PendingDelete bool   `json:"pending_delete"`
	State         string `json:"state"`
	Message       string `json:"message"`
	NeedsAdmin    bool   `json:"needs_admin"`
}

type ServiceActionResult struct {
	OK             bool   `json:"ok"`
	Message        string `json:"message"`
	NeedsElevation bool   `json:"needs_elevation,omitempty"`
}
