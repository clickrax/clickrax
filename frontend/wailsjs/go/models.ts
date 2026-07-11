export namespace models {
	
	export class SMTPSettings {
	    host?: string;
	    port?: number;
	    username?: string;
	    from?: string;
	    to?: string;
	    insecure_tls?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SMTPSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.insecure_tls = source["insecure_tls"];
	    }
	}
	export class AppSettings {
	    language: string;
	    start_with_windows: boolean;
	    minimize_to_tray: boolean;
	    default_server_id?: string;
	    default_exclusions?: string[];
	    bandwidth_mbps: number;
	    chunk_workers: number;
	    network_timeout_sec: number;
	    network_retries: number;
	    skip_behavior: string;
	    critical_error_limit: number;
	    restore_overwrite: string;
	    log_level: string;
	    check_updates: boolean;
	    webhook_url?: string;
	    smtp?: SMTPSettings;
	    notify_backup?: string;
	    notify_restore?: string;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.start_with_windows = source["start_with_windows"];
	        this.minimize_to_tray = source["minimize_to_tray"];
	        this.default_server_id = source["default_server_id"];
	        this.default_exclusions = source["default_exclusions"];
	        this.bandwidth_mbps = source["bandwidth_mbps"];
	        this.chunk_workers = source["chunk_workers"];
	        this.network_timeout_sec = source["network_timeout_sec"];
	        this.network_retries = source["network_retries"];
	        this.skip_behavior = source["skip_behavior"];
	        this.critical_error_limit = source["critical_error_limit"];
	        this.restore_overwrite = source["restore_overwrite"];
	        this.log_level = source["log_level"];
	        this.check_updates = source["check_updates"];
	        this.webhook_url = source["webhook_url"];
	        this.smtp = this.convertValues(source["smtp"], SMTPSettings);
	        this.notify_backup = source["notify_backup"];
	        this.notify_restore = source["notify_restore"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BackupCheckpoint {
	    job_id: string;
	    job_name: string;
	    phase: string;
	    new_chunks: number;
	    reused_chunks: number;
	    error?: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupCheckpoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	        this.job_name = source["job_name"];
	        this.phase = source["phase"];
	        this.new_chunks = source["new_chunks"];
	        this.reused_chunks = source["reused_chunks"];
	        this.error = source["error"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class BackupDestination {
	    id: string;
	    type: string;
	    name: string;
	    description?: string;
	    url?: string;
	    fingerprint?: string;
	    datastore?: string;
	    namespace?: string;
	    token_id?: string;
	    host?: string;
	    port?: number;
	    remote_path?: string;
	    domain?: string;
	    username?: string;
	    share?: string;
	    tls?: boolean;
	    passive?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BackupDestination(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.url = source["url"];
	        this.fingerprint = source["fingerprint"];
	        this.datastore = source["datastore"];
	        this.namespace = source["namespace"];
	        this.token_id = source["token_id"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.remote_path = source["remote_path"];
	        this.domain = source["domain"];
	        this.username = source["username"];
	        this.share = source["share"];
	        this.tls = source["tls"];
	        this.passive = source["passive"];
	    }
	}
	export class Schedule {
	    enabled: boolean;
	    type: string;
	    time?: string;
	    times?: string[];
	    weekdays?: number[];
	    run_on_startup: boolean;
	    skip_if_running: boolean;
	    full_backup_mode?: string;
	    full_backup_weekday?: number;
	    full_backup_anchor?: string;
	
	    static createFrom(source: any = {}) {
	        return new Schedule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.type = source["type"];
	        this.time = source["time"];
	        this.times = source["times"];
	        this.weekdays = source["weekdays"];
	        this.run_on_startup = source["run_on_startup"];
	        this.skip_if_running = source["skip_if_running"];
	        this.full_backup_mode = source["full_backup_mode"];
	        this.full_backup_weekday = source["full_backup_weekday"];
	        this.full_backup_anchor = source["full_backup_anchor"];
	    }
	}
	export class BackupJob {
	    id: string;
	    name: string;
	    destination_id?: string;
	    server_id?: string;
	    source_mode?: string;
	    sources: string[];
	    exclusions?: string[];
	    backup_id: string;
	    vss_enabled: boolean;
	    split_enabled: boolean;
	    split_size_gb: number;
	    skip_access_errors: boolean;
	    low_priority_io: boolean;
	    encryption_enabled: boolean;
	    schedule: Schedule;
	    verify_after_backup: boolean;
	    comment?: string;
	    notify_backup?: string;
	    notify_restore?: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.destination_id = source["destination_id"];
	        this.server_id = source["server_id"];
	        this.source_mode = source["source_mode"];
	        this.sources = source["sources"];
	        this.exclusions = source["exclusions"];
	        this.backup_id = source["backup_id"];
	        this.vss_enabled = source["vss_enabled"];
	        this.split_enabled = source["split_enabled"];
	        this.split_size_gb = source["split_size_gb"];
	        this.skip_access_errors = source["skip_access_errors"];
	        this.low_priority_io = source["low_priority_io"];
	        this.encryption_enabled = source["encryption_enabled"];
	        this.schedule = this.convertValues(source["schedule"], Schedule);
	        this.verify_after_backup = source["verify_after_backup"];
	        this.comment = source["comment"];
	        this.notify_backup = source["notify_backup"];
	        this.notify_restore = source["notify_restore"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PBSServer {
	    id: string;
	    name: string;
	    url: string;
	    fingerprint: string;
	    datastore: string;
	    namespace: string;
	    token_id: string;
	    description?: string;
	
	    static createFrom(source: any = {}) {
	        return new PBSServer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	        this.fingerprint = source["fingerprint"];
	        this.datastore = source["datastore"];
	        this.namespace = source["namespace"];
	        this.token_id = source["token_id"];
	        this.description = source["description"];
	    }
	}
	export class Config {
	    version: number;
	    destinations?: BackupDestination[];
	    servers?: PBSServer[];
	    jobs: BackupJob[];
	    settings: AppSettings;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.destinations = this.convertValues(source["destinations"], BackupDestination);
	        this.servers = this.convertValues(source["servers"], PBSServer);
	        this.jobs = this.convertValues(source["jobs"], BackupJob);
	        this.settings = this.convertValues(source["settings"], AppSettings);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ConnectionTestResult {
	    ok: boolean;
	    message: string;
	    pbs_version?: string;
	    datastores?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ConnectionTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.pbs_version = source["pbs_version"];
	        this.datastores = source["datastores"];
	    }
	}
	export class ContactInfo {
	    author_name: string;
	    copyright: string;
	    distribution_notice: string;
	    telegram_username: string;
	    telegram_handle: string;
	    telegram_url: string;
	    github_url: string;
	
	    static createFrom(source: any = {}) {
	        return new ContactInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.author_name = source["author_name"];
	        this.copyright = source["copyright"];
	        this.distribution_notice = source["distribution_notice"];
	        this.telegram_username = source["telegram_username"];
	        this.telegram_handle = source["telegram_handle"];
	        this.telegram_url = source["telegram_url"];
	        this.github_url = source["github_url"];
	    }
	}
	export class ExecutionRun {
	    job_id: string;
	    job_name: string;
	    trigger: string;
	    phase: string;
	    backup_type?: string;
	    percent: number;
	    bytes_transferred: number;
	    bytes_reused: number;
	    speed_bps: number;
	    eta_sec: number;
	    chunks_new: number;
	    chunks_reused: number;
	    files_done: number;
	    files_total: number;
	    files_skipped: number;
	    current_path?: string;
	    message: string;
	    started_at: string;
	    updated_at?: string;
	    can_stop: boolean;
	    can_retry: boolean;
	    can_dismiss: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ExecutionRun(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	        this.job_name = source["job_name"];
	        this.trigger = source["trigger"];
	        this.phase = source["phase"];
	        this.backup_type = source["backup_type"];
	        this.percent = source["percent"];
	        this.bytes_transferred = source["bytes_transferred"];
	        this.bytes_reused = source["bytes_reused"];
	        this.speed_bps = source["speed_bps"];
	        this.eta_sec = source["eta_sec"];
	        this.chunks_new = source["chunks_new"];
	        this.chunks_reused = source["chunks_reused"];
	        this.files_done = source["files_done"];
	        this.files_total = source["files_total"];
	        this.files_skipped = source["files_skipped"];
	        this.current_path = source["current_path"];
	        this.message = source["message"];
	        this.started_at = source["started_at"];
	        this.updated_at = source["updated_at"];
	        this.can_stop = source["can_stop"];
	        this.can_retry = source["can_retry"];
	        this.can_dismiss = source["can_dismiss"];
	    }
	}
	export class JobRunRecord {
	    job_id: string;
	    job_name: string;
	    status: string;
	    backup_type: string;
	    trigger?: string;
	    started_at: string;
	    finished_at: string;
	    duration_sec: number;
	    bytes_transferred: number;
	    bytes_reused: number;
	    files_total: number;
	    files_skipped: number;
	    snapshot?: string;
	    error?: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new JobRunRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	        this.job_name = source["job_name"];
	        this.status = source["status"];
	        this.backup_type = source["backup_type"];
	        this.trigger = source["trigger"];
	        this.started_at = source["started_at"];
	        this.finished_at = source["finished_at"];
	        this.duration_sec = source["duration_sec"];
	        this.bytes_transferred = source["bytes_transferred"];
	        this.bytes_reused = source["bytes_reused"];
	        this.files_total = source["files_total"];
	        this.files_skipped = source["files_skipped"];
	        this.snapshot = source["snapshot"];
	        this.error = source["error"];
	        this.message = source["message"];
	    }
	}
	export class ScheduledRunInfo {
	    job_id: string;
	    job_name: string;
	    run_at: string;
	    backup_type: string;
	    times_label: string;
	
	    static createFrom(source: any = {}) {
	        return new ScheduledRunInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	        this.job_name = source["job_name"];
	        this.run_at = source["run_at"];
	        this.backup_type = source["backup_type"];
	        this.times_label = source["times_label"];
	    }
	}
	export class QueuedRunInfo {
	    job_id: string;
	    job_name: string;
	    trigger: string;
	    position: number;
	    enqueued_at: string;
	
	    static createFrom(source: any = {}) {
	        return new QueuedRunInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	        this.job_name = source["job_name"];
	        this.trigger = source["trigger"];
	        this.position = source["position"];
	        this.enqueued_at = source["enqueued_at"];
	    }
	}
	export class ExecutionState {
	    active: ExecutionRun[];
	    interrupted: ExecutionRun[];
	    queued: QueuedRunInfo[];
	    upcoming: ScheduledRunInfo[];
	    recent_manual: JobRunRecord[];
	    recent_scheduled: JobRunRecord[];
	
	    static createFrom(source: any = {}) {
	        return new ExecutionState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = this.convertValues(source["active"], ExecutionRun);
	        this.interrupted = this.convertValues(source["interrupted"], ExecutionRun);
	        this.queued = this.convertValues(source["queued"], QueuedRunInfo);
	        this.upcoming = this.convertValues(source["upcoming"], ScheduledRunInfo);
	        this.recent_manual = this.convertValues(source["recent_manual"], JobRunRecord);
	        this.recent_scheduled = this.convertValues(source["recent_scheduled"], JobRunRecord);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HealthCheck {
	    name: string;
	    ok: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new HealthCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ok = source["ok"];
	        this.message = source["message"];
	    }
	}
	export class HealthReport {
	    checks: HealthCheck[];
	    ok: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HealthReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checks = this.convertValues(source["checks"], HealthCheck);
	        this.ok = source["ok"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class LastBackupInfo {
	    job_id: string;
	    job_name: string;
	    snapshot?: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new LastBackupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	        this.job_name = source["job_name"];
	        this.snapshot = source["snapshot"];
	        this.status = source["status"];
	    }
	}
	
	export class PathEstimate {
	    path: string;
	    files: number;
	    bytes: number;
	    approx?: boolean;
	    volume?: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PathEstimate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.files = source["files"];
	        this.bytes = source["bytes"];
	        this.approx = source["approx"];
	        this.volume = source["volume"];
	        this.error = source["error"];
	    }
	}
	export class ProgressEvent {
	    job_id: string;
	    job_name: string;
	    phase: string;
	    backup_type?: string;
	    percent: number;
	    bytes_transferred: number;
	    bytes_reused: number;
	    bytes_total_estimate: number;
	    chunks_new: number;
	    chunks_reused: number;
	    chunks_total: number;
	    speed_bps: number;
	    eta_sec: number;
	    started_at?: string;
	    current_path: string;
	    files_done: number;
	    files_total: number;
	    files_skipped: number;
	    files_changed: number;
	    message: string;
	    trigger?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProgressEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	        this.job_name = source["job_name"];
	        this.phase = source["phase"];
	        this.backup_type = source["backup_type"];
	        this.percent = source["percent"];
	        this.bytes_transferred = source["bytes_transferred"];
	        this.bytes_reused = source["bytes_reused"];
	        this.bytes_total_estimate = source["bytes_total_estimate"];
	        this.chunks_new = source["chunks_new"];
	        this.chunks_reused = source["chunks_reused"];
	        this.chunks_total = source["chunks_total"];
	        this.speed_bps = source["speed_bps"];
	        this.eta_sec = source["eta_sec"];
	        this.started_at = source["started_at"];
	        this.current_path = source["current_path"];
	        this.files_done = source["files_done"];
	        this.files_total = source["files_total"];
	        this.files_skipped = source["files_skipped"];
	        this.files_changed = source["files_changed"];
	        this.message = source["message"];
	        this.trigger = source["trigger"];
	    }
	}
	
	export class QuickBackupRequest {
	    name: string;
	    destination_id?: string;
	    server_id?: string;
	    sources: string[];
	    vss_enabled: boolean;
	    backup_id: string;
	    source_mode?: string;
	    force_full: boolean;
	    exclusions?: string[];
	    comment?: string;
	
	    static createFrom(source: any = {}) {
	        return new QuickBackupRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.destination_id = source["destination_id"];
	        this.server_id = source["server_id"];
	        this.sources = source["sources"];
	        this.vss_enabled = source["vss_enabled"];
	        this.backup_id = source["backup_id"];
	        this.source_mode = source["source_mode"];
	        this.force_full = source["force_full"];
	        this.exclusions = source["exclusions"];
	        this.comment = source["comment"];
	    }
	}
	export class RestoreBatchRequest {
	    job_id: string;
	    snapshot: string;
	    paths: string[];
	    dest_path: string;
	    to_original: boolean;
	    overwrite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RestoreBatchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	        this.snapshot = source["snapshot"];
	        this.paths = source["paths"];
	        this.dest_path = source["dest_path"];
	        this.to_original = source["to_original"];
	        this.overwrite = source["overwrite"];
	    }
	}
	export class RestoreFolderRequest {
	    job_id: string;
	    snapshot: string;
	    folder_path: string;
	    dest_path: string;
	    to_original: boolean;
	    overwrite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RestoreFolderRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	        this.snapshot = source["snapshot"];
	        this.folder_path = source["folder_path"];
	        this.dest_path = source["dest_path"];
	        this.to_original = source["to_original"];
	        this.overwrite = source["overwrite"];
	    }
	}
	export class RestoreRequest {
	    job_id: string;
	    snapshot: string;
	    file_path: string;
	    dest_path: string;
	    to_original: boolean;
	    overwrite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RestoreRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	        this.snapshot = source["snapshot"];
	        this.file_path = source["file_path"];
	        this.dest_path = source["dest_path"];
	        this.to_original = source["to_original"];
	        this.overwrite = source["overwrite"];
	    }
	}
	
	
	
	export class ServiceActionResult {
	    ok: boolean;
	    message: string;
	    needs_elevation?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ServiceActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.needs_elevation = source["needs_elevation"];
	    }
	}
	export class ServiceStatus {
	    installed: boolean;
	    running: boolean;
	    pending_delete: boolean;
	    state: string;
	    message: string;
	    needs_admin: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ServiceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.running = source["running"];
	        this.pending_delete = source["pending_delete"];
	        this.state = source["state"];
	        this.message = source["message"];
	        this.needs_admin = source["needs_admin"];
	    }
	}
	export class SnapshotFile {
	    path: string;
	    size: number;
	    is_dir: boolean;
	    modified?: string;
	    owner?: string;
	    attributes?: string;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.size = source["size"];
	        this.is_dir = source["is_dir"];
	        this.modified = source["modified"];
	        this.owner = source["owner"];
	        this.attributes = source["attributes"];
	    }
	}
	export class SnapshotInfo {
	    time: string;
	    backup: string;
	    backup_time: number;
	    comment?: string;
	    has_catalog: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.backup = source["backup"];
	        this.backup_time = source["backup_time"];
	        this.comment = source["comment"];
	        this.has_catalog = source["has_catalog"];
	    }
	}
	export class UpdateInfo {
	    current_version: string;
	    latest_version: string;
	    update_available: boolean;
	    url?: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current_version = source["current_version"];
	        this.latest_version = source["latest_version"];
	        this.update_available = source["update_available"];
	        this.url = source["url"];
	        this.message = source["message"];
	    }
	}
	export class VolumeFolder {
	    name: string;
	    path: string;
	    system: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VolumeFolder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.system = source["system"];
	    }
	}

}

