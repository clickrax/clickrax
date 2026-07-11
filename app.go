package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pbs-win-backup/internal/appstore"
	"pbs-win-backup/internal/backuprunner"
	"pbs-win-backup/internal/branding"
	"pbs-win-backup/internal/backup"
	"pbs-win-backup/internal/backupcancel"
	"pbs-win-backup/internal/backupqueue"
	"pbs-win-backup/internal/backuproot"
	"pbs-win-backup/internal/backuplock"
	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/credential"
	"pbs-win-backup/internal/destination"
	"pbs-win-backup/internal/eventlog"
	"pbs-win-backup/internal/health"
	"pbs-win-backup/internal/history"
	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/notify"
	"pbs-win-backup/internal/paths"
	"pbs-win-backup/internal/pbs"
	"pbs-win-backup/internal/pbsbackup"
	"pbs-win-backup/internal/restore"
	"pbs-win-backup/internal/schedule"
	"pbs-win-backup/internal/service"
	"pbs-win-backup/internal/updates"
	"pbs-win-backup/internal/version"
	"pbs-win-backup/internal/winutil"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	serviceStoppedWarnAt time.Time
	serviceStoppedWarnMu   sync.Mutex
)

type App struct {
	ctx                 context.Context
	store               *appstore.Store
	mu                  sync.RWMutex
	engine              *backup.Engine
	restore             *restoreController
	history             []models.JobRunResult
	lastProgress        models.ProgressEvent
	remoteStoppingJobID string
	remoteStoppingSince time.Time
}

func NewApp() *App {
	return &App{
		history: []models.JobRunResult{},
	}
}

func (a *App) bundle() *i18n.Bundle {
	return i18n.New(a.store.Settings().Language)
}

func (a *App) localizeConnectionResult(r models.ConnectionTestResult) models.ConnectionTestResult {
	r.Message = a.bundle().RetranslateStored(r.Message)
	return r
}

func (a *App) localizeProgress(ev models.ProgressEvent) models.ProgressEvent {
	b := a.bundle()
	ev.Message = b.RetranslateStored(ev.Message)
	ev.JobName = b.RetranslateJobName(ev.JobName)
	return ev
}

func mergeTerminalProgress(prev, ev models.ProgressEvent) models.ProgressEvent {
	if prev.JobID == "" || ev.JobID == "" || prev.JobID != ev.JobID {
		return ev
	}
	switch ev.Phase {
	case models.PhaseCancelled, models.PhaseError:
	default:
		return ev
	}
	if ev.Phase != models.PhaseCancelled {
		if ev.Percent == 0 && prev.Percent > 0 {
			ev.Percent = prev.Percent
		}
	}
	if ev.BytesTransferred == 0 && prev.BytesTransferred > 0 {
		ev.BytesTransferred = prev.BytesTransferred
	}
	if ev.BytesReused == 0 && prev.BytesReused > 0 {
		ev.BytesReused = prev.BytesReused
	}
	if ev.BytesTotal == 0 && prev.BytesTotal > 0 {
		ev.BytesTotal = prev.BytesTotal
	}
	if ev.FilesDone == 0 && prev.FilesDone > 0 {
		ev.FilesDone = prev.FilesDone
	}
	if ev.FilesTotal == 0 && prev.FilesTotal > 0 {
		ev.FilesTotal = prev.FilesTotal
	}
	if ev.FilesSkipped == 0 && prev.FilesSkipped > 0 {
		ev.FilesSkipped = prev.FilesSkipped
	}
	if ev.FilesChanged == 0 && prev.FilesChanged > 0 {
		ev.FilesChanged = prev.FilesChanged
	}
	if ev.ChunksNew == 0 && prev.ChunksNew > 0 {
		ev.ChunksNew = prev.ChunksNew
	}
	if ev.ChunksReused == 0 && prev.ChunksReused > 0 {
		ev.ChunksReused = prev.ChunksReused
	}
	if ev.SpeedBps == 0 && prev.SpeedBps > 0 {
		ev.SpeedBps = prev.SpeedBps
	}
	if ev.BackupType == "" {
		ev.BackupType = prev.BackupType
	}
	if ev.StartedAt == "" {
		ev.StartedAt = prev.StartedAt
	}
	if ev.Trigger == "" {
		ev.Trigger = prev.Trigger
	}
	return ev
}

func (a *App) emitProgress(ev models.ProgressEvent) {
	if a.ctx == nil {
		return
	}
	ev = a.localizeProgress(ev)
	a.mu.Lock()
	if a.lastProgress.JobID != "" && ev.JobID == a.lastProgress.JobID {
		ev = mergeTerminalProgress(a.lastProgress, ev)
	}
	a.lastProgress = ev
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "progress", ev)
}

func friendlyBackupError(b *i18n.Bundle, err error) string {
	return backuprunner.FriendlyError(b, err)
}

func shortenErr(err error) string {
	return backuprunner.ShortenErr(err)
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	cfg, err := config.LoadResilient()
	if err != nil {
		runtime.LogError(ctx, "config load: "+err.Error())
		cfg = config.DefaultConfig()
	}
	a.store = appstore.New(cfg)
	pbsbackup.SetChunkWorkersSetting(cfg.Settings.ChunkWorkers)
	a.migrateServerSecrets()

	if records, err := history.Load(); err == nil {
		a.history = records
	}

	a.engine = backup.NewEngine(func(ev models.ProgressEvent) {
		a.emitProgress(ev)
	})
	_ = backuplock.ClearStale()
	service.ClearStaleScheduleClaims()
	_ = backupqueue.ReconcileInflight()
	backupcancel.ReapStale(0)
	paths.EnsureSharedDataAccess()
	go a.processBackupQueue()
	go a.runScheduleLoop(ctx)
}

func (a *App) migrateServerSecrets() {
	dests := a.store.ListDestinations()
	ids := make([]string, 0, len(dests))
	for _, d := range dests {
		ids = append(ids, d.ID)
	}
	credential.MigrateSecrets(ids)
	credential.MigrateSMTPPassword()
	jobIDs := make([]string, 0, len(a.store.ListJobs()))
	for _, j := range a.store.ListJobs() {
		if j.EncryptionEnabled {
			jobIDs = append(jobIDs, j.ID)
		}
	}
	credential.MigratePassphrases(jobIDs)
}

func (a *App) ServerSecretReady(serverID string) bool {
	return credential.HasSecret(serverID)
}

func (a *App) GetConfigPath() (string, error) {
	return paths.ConfigPath()
}

// TestDestinationConnection checks connectivity (full PBS probe when type is pbs).
func (a *App) TestDestinationConnection(dest models.BackupDestination, secret string) (r models.ConnectionTestResult) {
	defer func() { r = a.localizeConnectionResult(r) }()
	b := a.bundle()
	if secret == "" && dest.ID != "" {
		var err error
		secret, err = credential.GetSecret(dest.ID)
		if err != nil {
			if dest.IsPBS() {
				return models.ConnectionTestResult{OK: false, Message: b.T("test.need_secret")}
			}
			if dest.Username != "" && dest.Username != "anonymous" {
				return models.ConnectionTestResult{OK: false, Message: b.T("test.need_password")}
			}
		}
	}
	if dest.IsPBS() {
		result := destination.Test(dest, secret)
		if !result.OK {
			return result
		}
		if a.engine != nil && a.engine.IsRunning() {
			result.Message = result.Message + b.T("test.protocol_not_checked")
			return result
		}
		if err := pbsbackup.ProbeBackupAccess(dest.ToPBSServer(), secret, backup.Hostname()); err != nil {
			return models.ConnectionTestResult{
				OK:         false,
				Message:    friendlyBackupError(b, err),
				PBSVersion: result.PBSVersion,
			}
		}
		return result
	}
	return destination.Test(dest, secret)
}

func (a *App) TestDestinationByID(destID string) (r models.ConnectionTestResult) {
	defer func() { r = a.localizeConnectionResult(r) }()
	a.mu.RLock()
	dest, ok := models.FindDestination(a.store.ConfigSnapshot(), destID)
	a.mu.RUnlock()
	if !ok {
		return models.ConnectionTestResult{OK: false, Message: a.bundle().T("test.dest_not_found")}
	}
	if dest.IsPBS() {
		return a.testServerREST(dest.ToPBSServer(), "")
	}
	secret, err := credential.GetSecret(dest.ID)
	if err != nil {
		return models.ConnectionTestResult{OK: false, Message: a.bundle().T("test.password_not_saved")}
	}
	return destination.Test(*dest, secret)
}

func (a *App) testServerREST(server models.PBSServer, secret string) models.ConnectionTestResult {
	b := a.bundle()
	if secret == "" {
		var err error
		secret, err = credential.GetSecret(server.ID)
		if err != nil {
			return models.ConnectionTestResult{OK: false, Message: b.T("test.need_secret")}
		}
	}
	return pbs.NewClient(server, secret).TestConnection()
}

// TestServerConnection checks REST API and backup protocol (use from «Проверить» in server form).
func (a *App) TestServerConnection(server models.PBSServer, secret string) (r models.ConnectionTestResult) {
	defer func() { r = a.localizeConnectionResult(r) }()
	b := a.bundle()
	result := a.testServerREST(server, secret)
	if !result.OK {
		return result
	}
	if a.engine != nil && a.engine.IsRunning() {
		result.Message = result.Message + b.T("test.protocol_not_checked")
		return result
	}
	backupID := backup.Hostname()
	if err := pbsbackup.ProbeBackupAccess(server, secret, backupID); err != nil {
		return models.ConnectionTestResult{
			OK:         false,
			Message:    friendlyBackupError(b, err),
			PBSVersion: result.PBSVersion,
		}
	}
	return result
}

// TestServerByID is a lightweight online check (REST only for PBS) for status badges.
func (a *App) TestServerByID(serverID string) models.ConnectionTestResult {
	return a.TestDestinationByID(serverID)
}

func (a *App) FetchFingerprint(url string) (string, error) {
	return pbs.FetchServerCertificateFingerprint(url)
}

func (a *App) emitLocalizedProgress(_ string) {
	if a.engine == nil || !a.engine.IsRunning() {
		return
	}
	a.mu.RLock()
	lp := a.lastProgress
	a.mu.RUnlock()
	if lp.JobID == "" || lp.Phase == models.PhaseIdle {
		return
	}
	a.emitProgress(lp)
}

func (a *App) HasSMTPPassword() bool {
	return credential.HasSMTPPassword()
}

func (a *App) TestSMTP(settings models.AppSettings, smtpPassword string) models.ConnectionTestResult {
	b := i18n.New(settings.Language)
	pw := strings.TrimSpace(smtpPassword)
	if pw == "" {
		var err error
		pw, err = credential.GetSMTPPassword()
		if err != nil {
			return models.ConnectionTestResult{OK: false, Message: b.T("test.need_smtp_password")}
		}
	}
	if err := notify.SendTestEmail(settings, pw); err != nil {
		return models.ConnectionTestResult{OK: false, Message: err.Error()}
	}
	return models.ConnectionTestResult{OK: true, Message: b.T("test.ok_email_sent")}
}

func (a *App) GetDefaultExclusions() []string {
	return config.DefaultExclusions()
}

func (a *App) GetHostname() string {
	return backup.Hostname()
}

func (a *App) StartBackup(jobID string) error {
	return a.startBackup(jobID, false)
}

func (a *App) StartForceFullBackup(jobID string) error {
	return a.startBackup(jobID, true)
}

func (a *App) SaveJobAndRun(job models.BackupJob, passphrase string, forceFull bool) error {
	if err := a.SaveJob(job, passphrase); err != nil {
		return err
	}
	a.mu.RLock()
	var savedID string
	for i := range a.store.ConfigSnapshot().Jobs {
		if job.ID != "" && a.store.ConfigSnapshot().Jobs[i].ID == job.ID {
			savedID = a.store.ConfigSnapshot().Jobs[i].ID
			break
		}
	}
	if savedID == "" {
		for i := range a.store.ConfigSnapshot().Jobs {
			if a.store.ConfigSnapshot().Jobs[i].Name == job.Name && a.store.ConfigSnapshot().Jobs[i].EffectiveDestinationID() == job.EffectiveDestinationID() {
				savedID = a.store.ConfigSnapshot().Jobs[i].ID
			}
		}
	}
	a.mu.RUnlock()
	if savedID == "" {
		return a.bundle().E("job.quick_not_found")
	}
	return a.startBackup(savedID, forceFull)
}

func (a *App) RunQuickBackup(req models.QuickBackupRequest) error {
	b := a.bundle()
	if len(req.Sources) == 0 {
		return b.E("quick.need_sources")
	}
	destID := req.DestinationID
	if destID == "" {
		destID = req.ServerID
	}
	if destID == "" {
		return b.E("quick.need_destination")
	}

	a.mu.RLock()
	dest, destOK := models.FindDestination(a.store.ConfigSnapshot(), destID)
	a.mu.RUnlock()
	if !destOK {
		return b.Ef("quick.dest_unavailable", map[string]string{"message": b.T("test.dest_not_found")})
	}
	test := a.localizeConnectionResult(a.TestDestinationConnection(*dest, ""))
	if !test.OK {
		return b.Ef("quick.dest_unavailable", map[string]string{"message": test.Message})
	}

	sources := make([]string, 0, len(req.Sources))
	for _, s := range req.Sources {
		if p := backuproot.NormalizeSourcePath(s); p != "" {
			sources = append(sources, p)
		}
	}
	if len(sources) == 0 {
		return b.E("job.need_paths")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = b.Tf("quick.name_prefix", map[string]string{"time": time.Now().Format("02.01.2006 15:04")})
	}
	backupID := req.BackupID
	if backupID == "" {
		backupID = backup.Hostname()
	}

	sourceMode := req.SourceMode
	if sourceMode == "" {
		sourceMode = "paths"
	}

	job := models.BackupJob{
		ID:               config.NewJobID(),
		Name:             name,
		DestinationID:    destID,
		ServerID:         destID,
		SourceMode:       sourceMode,
		Sources:          sources,
		Exclusions:       req.Exclusions,
		BackupID:         backupID,
		VSSEnabled:       req.VSSEnabled,
		SkipAccessErrors: true,
		Comment:          req.Comment,
		Schedule:         models.Schedule{Enabled: false},
	}

	if err := a.SaveJob(job, ""); err != nil {
		return err
	}
	return a.runJob(job, req.ForceFull)
}

func (a *App) PickFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: a.bundle().T("dialog.pick_folder"),
	})
}

func (a *App) PickRestoreFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: a.bundle().T("dialog.pick_restore_folder"),
	})
}

func (a *App) ListVolumes() []string {
	return backup.ListVolumes()
}

func (a *App) ListVolumeFolders(volume string) ([]models.VolumeFolder, error) {
	return backup.ListVolumeFolders(volume)
}

func (a *App) startBackup(jobID string, forceFull bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.store.Replace(cfg)
	a.mu.Unlock()

	a.mu.RLock()
	var job *models.BackupJob
	for i := range a.store.ConfigSnapshot().Jobs {
		if a.store.ConfigSnapshot().Jobs[i].ID == jobID {
			j := a.store.ConfigSnapshot().Jobs[i]
			job = &j
			break
		}
	}
	a.mu.RUnlock()

	if job == nil {
		return a.bundle().E("job.not_found")
	}
	return a.runJobAt(*job, forceFull, time.Time{})
}

func (a *App) runJob(job models.BackupJob, forceFull bool) error {
	return a.runJobAt(job, forceFull, time.Time{})
}

func (a *App) runJobAt(job models.BackupJob, forceFull bool, scheduledAt time.Time) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.store.Replace(cfg)
	a.mu.Unlock()

	jobID := job.ID
	a.mu.RLock()
	var fresh *models.BackupJob
	for i := range a.store.ConfigSnapshot().Jobs {
		if a.store.ConfigSnapshot().Jobs[i].ID == jobID {
			j := a.store.ConfigSnapshot().Jobs[i]
			fresh = &j
			break
		}
	}
	if fresh == nil {
		a.mu.RUnlock()
		return a.bundle().E("job.not_found")
	}
	destID := fresh.EffectiveDestinationID()
	dest, ok := models.FindDestination(a.store.ConfigSnapshot(), destID)
	a.mu.RUnlock()

	if !ok || dest == nil {
		return a.bundle().E("job.dest_not_found")
	}
	if len(fresh.Sources) == 0 {
		return a.bundle().E("job.no_sources")
	}

	item := backupqueue.ItemFromJob(*fresh, forceFull, scheduledAt)
	return a.submitBackup(item)
}

func (a *App) ClearBackupLock() models.ConnectionTestResult {
	b := a.bundle()
	if a.engine.IsRunning() {
		return models.ConnectionTestResult{OK: false, Message: b.T("test.backup_running")}
	}
	queueCleared := backupqueue.ClearStaleLock()
	if backuplock.ClearStale() {
		msg := b.T("lock.cleared_ready")
		if queueCleared {
			msg = b.T("lock.cleared_plural")
		}
		return models.ConnectionTestResult{OK: true, Message: msg}
	}
	if backuplock.ForceClearOwn() {
		return models.ConnectionTestResult{OK: true, Message: b.T("lock.cleared_own")}
	}
	if queueCleared {
		return models.ConnectionTestResult{OK: true, Message: b.T("lock.cleared_queue")}
	}
	return models.ConnectionTestResult{OK: false, Message: b.T("lock.held_other")}
}

func (a *App) StopBackup(jobID string) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		jobID = a.engine.CurrentJobID()
	}
	if jobID != "" && a.engine.IsRunning() && a.engine.CurrentJobID() == jobID {
		a.engine.Stop()
		a.emitStoppingProgress(jobID)
		return
	}
	if jobID == "" {
		return
	}
	if !a.isJobActivelyRunning(jobID) {
		backupcancel.Clear(jobID)
		a.clearRemoteStopping()
		return
	}
	if err := backupcancel.Request(jobID); err != nil {
		eventlog.Error("cancel request: " + err.Error())
		return
	}
	a.setRemoteStopping(jobID)
	a.emitStoppingProgress(jobID)
}

func (a *App) isJobActivelyRunning(jobID string) bool {
	if jobID == "" {
		return false
	}
	if a.engine.IsRunning() && a.engine.CurrentJobID() == jobID {
		return true
	}
	cps, _ := pbsbackup.ListActiveCheckpoints(activeCheckpointMaxAge)
	for _, cp := range cps {
		if cp.JobID == jobID {
			return true
		}
	}
	return false
}

func (a *App) setRemoteStopping(jobID string) {
	a.mu.Lock()
	a.remoteStoppingJobID = jobID
	a.remoteStoppingSince = time.Now()
	a.mu.Unlock()
}

func (a *App) clearRemoteStopping() {
	a.mu.Lock()
	a.remoteStoppingJobID = ""
	a.remoteStoppingSince = time.Time{}
	a.mu.Unlock()
}

func (a *App) emitStoppingProgress(jobID string) {
	a.mu.RLock()
	lp := a.lastProgress
	a.mu.RUnlock()
	ev := models.ProgressEvent{JobID: jobID, Phase: models.PhasePreparing}
	if lp.JobID == jobID {
		ev = lp
	}
	ev.Message = a.bundle().T("backup.stopping")
	a.emitProgress(ev)
}

// shutdown stops an active backup and waits for PBS session cleanup before exit.
func (a *App) shutdown(ctx context.Context) {
	if a.engine == nil {
		return
	}
	if !a.engine.IsRunning() {
		return
	}
	eventlog.Info("закрытие приложения: остановка активного бэкапа…")
	a.engine.Stop()
	deadline := time.Now().Add(60 * time.Second)
	for a.engine.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
	_ = backuplock.ForceClearOwn()
}

// ResumeBackup retries a job after an interrupted PBS backup (chunks on PBS are reused).
func (a *App) ResumeBackup(jobID string) error {
	return a.StartBackup(jobID)
}

// DismissBackupCheckpoint removes a stale interrupted-backup marker from the UI.
func (a *App) DismissBackupCheckpoint(jobID string) error {
	return pbsbackup.ClearCheckpoint(jobID)
}

func (a *App) IsStopping() bool {
	if a.engine.IsStopping() {
		return true
	}
	a.mu.RLock()
	jobID := a.remoteStoppingJobID
	since := a.remoteStoppingSince
	a.mu.RUnlock()
	if jobID == "" {
		return false
	}
	if !since.IsZero() && time.Since(since) > 2*time.Minute {
		a.clearRemoteStopping()
		return false
	}
	if a.isJobActivelyRunning(jobID) {
		return true
	}
	a.clearRemoteStopping()
	return false
}

func (a *App) EstimatePath(path string, jobExclusions []string) models.PathEstimate {
	b := a.bundle()
	path = backuproot.NormalizeSourcePath(path)
	if path == "" {
		return models.PathEstimate{Error: b.T("path.empty")}
	}
	a.mu.RLock()
	global := a.store.ConfigSnapshot().Settings.DefaultExclusions
	a.mu.RUnlock()
	scan, err := backup.ScanPath(path, global, jobExclusions)
	if err != nil {
		return models.PathEstimate{Path: path, Error: err.Error()}
	}
	return models.PathEstimate{
		Path:   path,
		Files:  scan.Files,
		Bytes:  scan.Bytes,
		Approx: scan.Approx,
		Volume: scan.Approx && backup.IsVolumeRoot(path),
	}
}

func (a *App) IsBackupRunning() bool {
	return a.engine.IsRunning()
}

func (a *App) GetLastProgress() models.ProgressEvent {
	a.mu.RLock()
	lp := a.lastProgress
	running := a.engine.IsRunning()
	history := a.history
	a.mu.RUnlock()

	if running {
		return a.localizeProgress(lp)
	}

	if lp.JobID != "" && isInProgressPhase(lp.Phase) {
		if a.isJobActivelyRunning(lp.JobID) {
			ev := lp
			if ev.Message == "" {
				ev.Message = a.bundle().T("backup.interrupted")
			}
			return a.localizeProgress(ev)
		}
	}

	switch lp.Phase {
	case models.PhaseDone, models.PhaseError, models.PhaseCancelled, models.PhaseIdle, "":
		if lp.Phase != "" {
			return lp
		}
	}

	if len(history) > 0 {
		b := a.bundle()
		h := history[0]
		phase := models.PhaseDone
		msg := b.Tf("backup.last_status", map[string]string{"status": h.Status})
		if h.Status == "cancelled" {
			phase = models.PhaseCancelled
			msg = b.T("backup.last_cancelled")
		} else if h.Status == "error" {
			phase = models.PhaseError
			msg = h.Error
			if msg == "" {
				msg = b.T("backup.last_error")
			}
		} else if h.Status == "ok" || h.Status == "warning" {
			msg = b.Tf("backup.toast_done", map[string]string{"type": h.BackupType})
		}
		return models.ProgressEvent{
			JobID:            h.JobID,
			JobName:          h.JobName,
			Phase:            phase,
			Percent:          100,
			BytesTransferred: h.BytesTransferred,
			BytesReused:      h.BytesReused,
			BackupType:       h.BackupType,
			Message:          msg,
		}
	}

	return models.ProgressEvent{Phase: models.PhaseIdle}
}

func (a *App) GetLastSuccessfulBackup() (models.LastBackupInfo, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, h := range a.history {
		if h.Status == "ok" || h.Status == "warning" {
			return models.LastBackupInfo{
				JobID:    h.JobID,
				JobName:  h.JobName,
				Snapshot: h.Snapshot,
				Status:   h.Status,
			}, nil
		}
	}
	return models.LastBackupInfo{}, a.bundle().E("backup.no_successful")
}

func (a *App) ensureHistoryLoaded() {
	a.mu.RLock()
	loaded := len(a.history) > 0
	a.mu.RUnlock()
	if loaded {
		return
	}
	if records, err := history.Load(); err == nil {
		a.mu.Lock()
		if len(a.history) == 0 {
			a.history = records
		}
		a.mu.Unlock()
	}
}

func (a *App) GetHistory() []models.JobRunRecord {
	a.ensureHistoryLoaded()
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]models.JobRunRecord, len(a.history))
	for i, h := range a.history {
		out[i] = localizeJobRecord(a.bundle(), h.ToRecord())
	}
	return out
}

func (a *App) ClearHistory() error {
	if err := history.Clear(); err != nil {
		return err
	}
	a.mu.Lock()
	a.history = nil
	a.mu.Unlock()
	return nil
}

func (a *App) ListSnapshots(jobID string) ([]models.SnapshotInfo, error) {
	a.mu.RLock()
	var job models.BackupJob
	for _, j := range a.store.ConfigSnapshot().Jobs {
		if j.ID == jobID {
			job = j
			break
		}
	}
	a.mu.RUnlock()

	svc := a.restoreService()
	return svc.ListSnapshots(job)
}

func (a *App) pbsServerResolver() func(string) (*models.PBSServer, error) {
	return func(id string) (*models.PBSServer, error) {
		a.mu.RLock()
		defer a.mu.RUnlock()
		dest, ok := models.FindDestination(a.store.ConfigSnapshot(), id)
		if !ok || !dest.IsPBS() {
			return nil, a.bundle().E("restore.dest_not_found")
		}
		s := dest.ToPBSServer()
		return &s, nil
	}
}

func (a *App) restoreService() *restore.Service {
	return restore.NewWithDestinations(a.pbsServerResolver(), a.jobDestinationResolver())
}

func (a *App) jobDestinationResolver() func(models.BackupJob) (*models.BackupDestination, string, error) {
	return func(job models.BackupJob) (*models.BackupDestination, string, error) {
		a.mu.RLock()
		defer a.mu.RUnlock()
		dest, ok := models.FindDestination(a.store.ConfigSnapshot(), job.EffectiveDestinationID())
		if !ok {
			return nil, "", a.bundle().E("restore.dest_not_found")
		}
		secret, err := credential.GetSecret(dest.ID)
		if err != nil {
			return nil, "", a.bundle().E("restore.secret_not_found")
		}
		return dest, secret, nil
	}
}

func (a *App) jobByID(jobID string) (models.BackupJob, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, j := range a.store.ConfigSnapshot().Jobs {
		if j.ID == jobID {
			return j, nil
		}
	}
	return models.BackupJob{}, a.bundle().E("job.not_found")
}

func (a *App) emitRestoreProgress(done, total int, path string) {
	percent := 0.0
	if total > 0 {
		percent = float64(done) / float64(total) * 100
	}
	runtime.EventsEmit(a.ctx, "restore-progress", map[string]interface{}{
		"files_done":   done,
		"files_total":  total,
		"current_path": path,
		"percent":      percent,
	})
}

func (a *App) streamRestoreProgress(done, total int, message string) {
	if total > 0 {
		a.emitRestoreProgress(done, total, message)
	} else {
		a.emitRestoreProgress(0, 1, message)
	}
}

func (a *App) appendRunHistory(result models.JobRunResult) {
	a.mu.Lock()
	a.history = append([]models.JobRunResult{result}, a.history...)
	if len(a.history) > 200 {
		a.history = a.history[:200]
	}
	a.mu.Unlock()
	_ = history.Append(result)
}

func (a *App) recordRestore(job models.BackupJob, snapshot string, count int, toOriginal bool, destPath, status, errMsg string, started time.Time) {
	b := a.bundle()
	destLabel := b.T("restore.original_path")
	if !toOriginal && destPath != "" {
		destLabel = destPath
	}
	msg := b.Tf("restore.restored_files", map[string]string{
		"count": fmt.Sprintf("%d", count),
		"dest":  destLabel,
	})
	if status != "ok" {
		if errMsg != "" {
			msg = errMsg
		} else {
			msg = b.T("restore.failed")
		}
	}
	finished := time.Now()
	result := models.JobRunResult{
		JobID:       job.ID,
		JobName:     job.Name,
		Status:      status,
		BackupType:  "restore",
		StartedAt:   started,
		FinishedAt:  finished,
		DurationSec: int64(finished.Sub(started).Seconds()),
		FilesTotal:  count,
		Snapshot:    snapshot,
		Error:       errMsg,
		Message:     msg,
	}
	a.appendRunHistory(result)
	if status == "ok" {
		eventlog.Info(fmt.Sprintf("восстановление %s: %s", job.Name, msg))
	} else {
		eventlog.Error(fmt.Sprintf("восстановление %s: %s", job.Name, msg))
	}
	a.sendRestoreEmail(job, result)
}

func (a *App) restoreFileSync(req models.RestoreRequest) models.RestoreResult {
	ctx := a.beginRestore()
	defer a.endRestore()
	job, err := a.jobByID(req.JobID)
	if err != nil {
		return models.RestoreResult{OK: false, Message: err.Error()}
	}
	svc := a.restoreService()
	for {
		result := svc.Restore(ctx, req, job, a.streamRestoreProgress)
		if !result.NeedsConfirm {
			return result
		}
		b := a.bundle()
		answer, _ := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.QuestionDialog,
			Title:   b.T("restore.file_exists_title"),
			Message: b.Tf("restore.file_exists_msg", map[string]string{"path": result.ExistingPath}),
		})
		if answer != "Yes" {
			return models.RestoreResult{OK: false, Message: b.T("restore.cancelled")}
		}
		req.Overwrite = true
	}
}

// RestoreFile runs restore in background; result arrives via restore-file-done event.
func (a *App) RestoreFile(req models.RestoreRequest) {
	go func() {
		started := time.Now()
		job, _ := a.jobByID(req.JobID)
		var result models.RestoreResult
		defer func() {
			if r := recover(); r != nil {
				result = models.RestoreResult{OK: false, Message: a.bundle().Tf("restore.restore_failed_wrap", map[string]string{"err": fmt.Sprintf("%v", r)})}
			}
			runtime.EventsEmit(a.ctx, "restore-file-done", result)
			count := 0
			if result.OK {
				count = 1
			}
			status := "ok"
			errMsg := ""
			if !result.OK {
				status, errMsg = restoreStatusFromMessage(result.OK, result.Message)
			}
			a.recordRestore(job, req.Snapshot, count, req.ToOriginal, req.DestPath, status, errMsg, started)
		}()
		result = a.restoreFileSync(req)
	}()
}

func (a *App) restoreFolderSync(req models.RestoreFolderRequest) models.RestoreFolderResult {
	ctx := a.beginRestore()
	defer a.endRestore()
	job, err := a.jobByID(req.JobID)
	if err != nil {
		return models.RestoreFolderResult{OK: false, Message: err.Error()}
	}
	svc := a.restoreService()
	for {
		result := svc.RestoreFolder(ctx, req, job, a.emitRestoreProgress)
		if result.OK {
			return result
		}
		if strings.Contains(result.Message, "file_exists:") && !req.Overwrite {
			b := a.bundle()
			answer, _ := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
				Type:    runtime.QuestionDialog,
				Title:   b.T("restore.file_exists_title"),
				Message: b.T("restore.files_exist_msg"),
			})
			if answer != "Yes" {
				return models.RestoreFolderResult{OK: false, Message: b.T("restore.cancelled"), Count: result.Count}
			}
			req.Overwrite = true
			continue
		}
		return result
	}
}

// RestoreFolder runs restore in background; result arrives via restore-folder-done event.
func (a *App) RestoreFolder(req models.RestoreFolderRequest) {
	go func() {
		started := time.Now()
		job, _ := a.jobByID(req.JobID)
		var result models.RestoreFolderResult
		defer func() {
			if r := recover(); r != nil {
				result = models.RestoreFolderResult{OK: false, Message: a.bundle().Tf("restore.restore_failed_wrap", map[string]string{"err": fmt.Sprintf("%v", r)})}
			}
			runtime.EventsEmit(a.ctx, "restore-folder-done", result)
			status := "ok"
			errMsg := ""
			if !result.OK {
				status, errMsg = restoreStatusFromMessage(result.OK, result.Message)
			}
			a.recordRestore(job, req.Snapshot, result.Count, req.ToOriginal, req.DestPath, status, errMsg, started)
		}()
		result = a.restoreFolderSync(req)
	}()
}

func (a *App) restoreBatchSync(req models.RestoreBatchRequest) models.RestoreBatchResult {
	ctx := a.beginRestore()
	defer a.endRestore()
	job, err := a.jobByID(req.JobID)
	if err != nil {
		return models.RestoreBatchResult{OK: false, Message: err.Error()}
	}
	svc := a.restoreService()
	for {
		result := svc.RestoreBatch(ctx, req, job, a.emitRestoreProgress)
		if result.OK {
			return result
		}
		if strings.Contains(result.Message, "file_exists:") && !req.Overwrite {
			b := a.bundle()
			answer, _ := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
				Type:    runtime.QuestionDialog,
				Title:   b.T("restore.file_exists_title"),
				Message: b.T("restore.files_exist_msg"),
			})
			if answer != "Yes" {
				return models.RestoreBatchResult{OK: false, Message: b.T("restore.cancelled"), Count: result.Count}
			}
			req.Overwrite = true
			continue
		}
		return result
	}
}

// RestoreBatch restores multiple files/folders; result via restore-batch-done event.
func (a *App) RestoreBatch(req models.RestoreBatchRequest) {
	go func() {
		started := time.Now()
		job, _ := a.jobByID(req.JobID)
		var result models.RestoreBatchResult
		defer func() {
			if r := recover(); r != nil {
				result = models.RestoreBatchResult{OK: false, Message: a.bundle().Tf("restore.restore_failed_wrap", map[string]string{"err": fmt.Sprintf("%v", r)})}
			}
			runtime.EventsEmit(a.ctx, "restore-batch-done", result)
			status := "ok"
			errMsg := ""
			if !result.OK {
				status, errMsg = restoreStatusFromMessage(result.OK, result.Message)
			}
			dest := req.DestPath
			a.recordRestore(job, req.Snapshot, result.Count, req.ToOriginal, dest, status, errMsg, started)
		}()
		result = a.restoreBatchSync(req)
	}()
}

func (a *App) GetBackupCheckpoint(jobID string) (models.BackupCheckpoint, error) {
	cp, err := pbsbackup.LoadCheckpoint(jobID)
	if err != nil {
		return models.BackupCheckpoint{}, err
	}
	if cp == nil {
		return models.BackupCheckpoint{}, nil
	}
	return models.BackupCheckpoint{
		JobID:        cp.JobID,
		JobName:      cp.JobName,
		Phase:        cp.Phase,
		NewChunks:    cp.NewChunks,
		ReusedChunks: cp.ReusedChunks,
		Error:        cp.Error,
		UpdatedAt:    cp.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}, nil
}

func (a *App) ListSnapshotFiles(jobID, snapshot, path, search string) ([]models.SnapshotFile, error) {
	a.mu.RLock()
	var job models.BackupJob
	for _, j := range a.store.ConfigSnapshot().Jobs {
		if j.ID == jobID {
			job = j
			break
		}
	}
	a.mu.RUnlock()

	svc := a.restoreService()
	return svc.ListFiles(job, snapshot, path, search)
}

func (a *App) GetPBSWebURL(jobID string) (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, j := range a.store.ConfigSnapshot().Jobs {
		if j.ID == jobID {
			dest, ok := models.FindDestination(a.store.ConfigSnapshot(), j.EffectiveDestinationID())
			if ok && dest.IsPBS() {
				return pbsbackupOpenURL(dest.ToPBSServer()), nil
			}
			return "", a.bundle().E("restore.pbs_only")
		}
	}
	return "", a.bundle().E("job.not_found")
}

func pbsbackupOpenURL(s models.PBSServer) string {
	return strings.TrimRight(s.URL, "/") + "/#Datastore/" + s.Datastore
}

func (a *App) OpenDataFolder() {
	dir, err := paths.DataDir()
	if err != nil {
		return
	}
	runtime.BrowserOpenURL(a.ctx, "file:///"+filepath.ToSlash(dir))
}

func (a *App) GetServiceStatus() models.ServiceStatus {
	st := service.QueryStatus()
	return models.ServiceStatus{
		Installed:     st.Installed,
		Running:       st.Running,
		PendingDelete: st.PendingDelete,
		State:         st.State,
		Message:       st.Message,
		NeedsAdmin:    !winutil.IsElevated(),
	}
}

func (a *App) InstallService() models.ServiceActionResult {
	return a.runServiceAction("install-service", "service.action.installed_started")
}

func (a *App) UninstallService() models.ServiceActionResult {
	return a.runServiceAction("uninstall-service", "service.action.uninstalled")
}

func (a *App) StartService() models.ServiceActionResult {
	return a.runServiceAction("start-service", "service.action.started")
}

func (a *App) StopService() models.ServiceActionResult {
	return a.runServiceAction("stop-service", "service.action.stopped")
}

func (a *App) RestartService() models.ServiceActionResult {
	return a.runServiceAction("restart-service", "service.action.restarted")
}

func (a *App) runServiceAction(flag, successKey string) models.ServiceActionResult {
	b := a.bundle()
	if winutil.IsElevated() {
		var err error
		switch flag {
		case "install-service":
			err = service.Install()
		case "uninstall-service":
			err = service.Uninstall()
		case "start-service":
			err = service.Start()
		case "stop-service":
			err = service.Stop()
		case "restart-service":
			err = service.Restart()
		default:
			return models.ServiceActionResult{OK: false, Message: b.T("service.action.unknown")}
		}
		if err != nil {
			return models.ServiceActionResult{OK: false, Message: err.Error()}
		}
		return models.ServiceActionResult{OK: true, Message: b.T(successKey)}
	}

	exe, err := os.Executable()
	if err != nil {
		return models.ServiceActionResult{OK: false, Message: err.Error()}
	}
	resultFile := filepath.Join(os.TempDir(), fmt.Sprintf("pbs-svc-%d.json", time.Now().UnixNano()))
	params := fmt.Sprintf(`--%s --result-file "%s"`, flag, resultFile)
	code, err := winutil.RunElevated(exe, params)
	if err != nil {
		return models.ServiceActionResult{OK: false, Message: err.Error(), NeedsElevation: true}
	}
	if data, readErr := os.ReadFile(resultFile); readErr == nil {
		_ = os.Remove(resultFile)
		var res models.ServiceActionResult
		if json.Unmarshal(data, &res) == nil && res.Message != "" {
			return res
		}
	}
	if code != 0 {
		return models.ServiceActionResult{
			OK:             false,
			Message:        b.Tf("service.action.failed_code", map[string]string{"code": fmt.Sprintf("%d", code)}),
			NeedsElevation: true,
		}
	}
	return models.ServiceActionResult{OK: true, Message: b.T(successKey)}
}

func (a *App) runScheduleLoop(ctx context.Context) {
	run := func() { a.runScheduledJobs(time.Now()) }
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func (a *App) runScheduledJobs(now time.Time) {
	if a.engine == nil {
		return
	}
	st := service.QueryStatus()
	// Defer to the Windows service only when it is installed, running, and not pending removal.
	if st.Installed && st.Running && !st.PendingDelete {
		return
	}
	if st.Installed && !st.Running && !st.PendingDelete {
		serviceStoppedWarnMu.Lock()
		if time.Since(serviceStoppedWarnAt) >= 15*time.Minute {
			serviceStoppedWarnAt = time.Now()
			eventlog.Warning(a.bundle().T("schedule.service_not_running"))
		}
		serviceStoppedWarnMu.Unlock()
	}
	cfg, err := config.Load()
	if err != nil {
		eventlog.Error("расписание: config: " + err.Error())
		return
	}
	a.mu.Lock()
	a.store.Replace(cfg)
	a.mu.Unlock()

	a.processBackupQueue()

	jobs := cfg.Jobs
	for _, job := range jobs {
		if !job.Schedule.Enabled {
			continue
		}
		if !service.MatchesWindow(job, now) {
			continue
		}
		if service.SlotAlreadySucceeded(job, now) {
			continue
		}
		item := backupqueue.ItemFromJob(job, schedule.ShouldForceFull(job.Schedule, now), now)
		if contained, err := backupqueue.Contains(item); err == nil && contained {
			continue
		}
		if !service.TryClaimSlot(job, now) {
			continue
		}
		if service.SlotAlreadySucceeded(job, now) {
			service.ReleaseSlotClaim(job, now)
			continue
		}
		if !service.BeginScheduledRun(job.ID, item.SlotKey) {
			service.ReleaseSlotClaim(job, now)
			continue
		}
		if err := a.submitBackup(item); err != nil {
			service.EndScheduledRun(job.ID, item.SlotKey)
			service.ReleaseSlotClaim(job, now)
			eventlog.Error("расписание " + job.Name + " (" + schedule.DescribeRun(job.Schedule, now) + "): " + err.Error())
		}
	}
}

func (a *App) GetDiagnostics() map[string]string {
	dataDir, _ := paths.DataDir()
	if dataDir == "" {
		path := os.Getenv("ProgramData")
		if path == "" {
			path = `C:\ProgramData`
		}
		dataDir = filepath.Join(path, paths.AppFolderName)
	}
	return map[string]string{
		"version":     version.Version,
		"author":      branding.AuthorName,
		"copyright":   branding.Copyright,
		"hostname":    backup.Hostname(),
		"data_dir":    dataDir,
		"go_version":  "1.26",
		"engine":      "proxmoxbackupclient_go (pbscommon)",
		"event_log":   "PbsWinBackup",
		"telegram":    branding.TelegramHandle,
		"github":      branding.GitHubURL,
		"distribution": branding.DistributionNotice,
	}
}

func (a *App) GetContactInfo() models.ContactInfo {
	return models.ContactInfo{
		AuthorName:         branding.AuthorName,
		Copyright:          branding.Copyright,
		DistributionNotice: branding.DistributionNotice,
		TelegramUsername:   branding.TelegramUsername,
		TelegramHandle:     branding.TelegramHandle,
		TelegramURL:        branding.TelegramURL,
		GitHubURL:          branding.GitHubURL,
	}
}

func (a *App) OpenTelegramContact() {
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, branding.TelegramURL)
	}
}

func (a *App) GetVersion() string {
	return version.Version
}

func (a *App) GetNextScheduledRun() string {
	a.mu.RLock()
	cfg := a.store.ConfigSnapshot()
	a.mu.RUnlock()
	if cfg == nil {
		return ""
	}
	item, ok := schedule.NextScheduled(cfg.Jobs, time.Now())
	if !ok {
		return ""
	}
	b := a.bundle()
	return item.RunAt + " (" + b.FormatBackupType(item.BackupType) + ")"
}

func (a *App) RunHealthCheck() models.HealthReport {
	a.mu.RLock()
	cfg := *a.store.ConfigSnapshot()
	a.mu.RUnlock()
	r := health.Run(&cfg)
	out := models.HealthReport{OK: r.OK}
	for _, c := range r.Checks {
		out.Checks = append(out.Checks, models.HealthCheck{
			Name: c.Name, OK: c.OK, Message: c.Message,
		})
	}
	return out
}

func (a *App) CheckForUpdates() models.UpdateInfo {
	r := updates.Check()
	return models.UpdateInfo{
		CurrentVersion:  r.CurrentVersion,
		LatestVersion:   r.LatestVersion,
		UpdateAvailable: r.UpdateAvailable,
		URL:             r.URL,
		Message:         r.Message,
	}
}

func (a *App) ExportConfigDialog() (string, error) {
	b := a.bundle()
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: branding.ExeName + "-config.json",
		Title:           b.T("dialog.export_config"),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON", Pattern: "*.json"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	a.mu.RLock()
	cfg := *a.store.ConfigSnapshot()
	a.mu.RUnlock()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) ImportConfigDialog() (string, error) {
	b := a.bundle()
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: b.T("dialog.import_config"),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON", Pattern: "*.json"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	const maxImportConfigBytes = 4 << 20
	if len(data) > maxImportConfigBytes {
		return "", b.E("validate.import_too_large")
	}
	var imported models.Config
	if err := json.Unmarshal(data, &imported); err != nil {
		return "", b.Ewrap("validate.json_invalid", map[string]string{"err": err.Error()}, err)
	}
	if imported.Settings.DefaultExclusions == nil {
		imported.Settings.DefaultExclusions = config.DefaultExclusions()
	}
	if err := notify.ValidateWebhookURL(imported.Settings.WebhookURL); err != nil {
		return "", err
	}
	for _, dest := range imported.Destinations {
		if dest.IsPBS() {
			if err := models.ValidatePBSURL(b, dest.URL); err != nil {
				return "", err
			}
		}
	}
	if err := config.Save(&imported); err != nil {
		return "", err
	}
	reloaded, err := config.Load()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	a.store.Replace(reloaded)
	a.mu.Unlock()
	return path, nil
}

func (a *App) ExportHistoryDialog() (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "pbs-backup-history.json",
		Title:           a.bundle().T("dialog.export_history"),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON", Pattern: "*.json"},
		},
	})
	if err != nil || path == "" {
		return "", err
	}
	records := a.GetHistory()
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}
