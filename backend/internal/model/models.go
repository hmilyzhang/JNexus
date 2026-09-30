// JNexus Ops Platform — By JJ Zhang, Version 1.0

package model

import (
	"encoding/json"
	"time"
)

// Roles
const (
	RoleAdmin     = "admin"     // Administrator
	RoleOps       = "ops"       // Ops
	RolePublisher = "publisher" // Publisher
	RoleViewer    = "viewer"    // Read-only
	RoleAuditor   = "auditor"   // Auditor: can view execution records and audit logs
	RoleK8s       = "k8s"       // K8s ops: cluster viewing plus Pod/cron job/service account operations
)

type User struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Username    string     `gorm:"uniqueIndex;size:64" json:"username"`
	DisplayName string     `gorm:"size:128" json:"display_name"` // Display name (synced from LDAP displayName/cn; local users can set it in their profile)
	Password    string     `json:"-"`
	Role        string     `gorm:"size:32;index" json:"role"`
	AuthSource  string     `gorm:"size:16;default:local" json:"auth_source"` // local / ldap
	Email       string     `gorm:"size:128" json:"email"`
	Status      int        `gorm:"default:1" json:"status"` // 1 enabled, 0 disabled
	MFAEnabled  bool       `gorm:"default:false" json:"mfa_enabled"`
	MFASecret   string     `gorm:"size:256" json:"-"` // TOTP secret, stored AES-GCM encrypted
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `gorm:"size:64" json:"created_by"` // Creator ("LDAP" for accounts auto-created via LDAP)
	UpdatedBy   string     `gorm:"size:64" json:"updated_by"` // Last modified by
	UpdatedAt   time.Time  `json:"updated_at"`
	DisabledAt  *time.Time `json:"disabled_at"`                // Disabled at (cleared once re-enabled)
	DisabledBy  string     `gorm:"size:64" json:"disabled_by"` // Disabled by
}

// SystemConfig system configuration (key-value; value may be JSON text)
type SystemConfig struct {
	Key   string `gorm:"primaryKey;size:64" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

type HostGroup struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"index;size:128" json:"name"` // unique per parent (filesystem-style), not globally
	ParentID    *uint     `gorm:"index" json:"parent_id"`     // Parent group (multi-level tree)
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type SSHKey struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"uniqueIndex;size:128" json:"name"`
	PublicKey string `json:"public_key"`
	// Private key, AES-GCM encrypted
	PrivateKey string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
}

type Host struct {
	LastBootID string `gorm:"size:64" json:"-"` // Last collected boot_id (used to detect system reboots)
	ID         uint   `gorm:"primaryKey" json:"id"`
	Name       string `gorm:"size:128" json:"name"`
	IP         string `gorm:"size:64;index" json:"ip"`
	Port       int    `json:"port"`
	OSType     string `gorm:"size:16;default:linux" json:"os_type"` // linux / windows
	WinRMPort  int    `json:"winrm_port"`                           // Windows: 5985(HTTP) / 5986(HTTPS)
	RDPPort    int    `json:"rdp_port"`                             // Windows: 3389
	// Kerberos auth (Windows, domain environments): preferred over NTLM for CIS-hardened
	// domains. Requires WinRM HTTPS (5986), a krb5.conf reachable by the server, and an SPN.
	WinRMKerberos bool       `json:"winrm_kerberos"`            // use Kerberos (gokrb5) instead of NTLM/Basic
	WinRMSPN      string     `gorm:"size:128" json:"winrm_spn"` // SPN override, default WSMAN/<name>
	Username      string     `gorm:"size:64" json:"username"`
	AuthType      string     `gorm:"size:16;default:key" json:"auth_type"` // key / password
	SSHKeyID      *uint      `json:"ssh_key_id"`
	Password      string     `json:"-"` // AES-GCM encrypted
	GroupID       *uint      `gorm:"index" json:"group_id"`
	// Cloud identity (CSP asset sync): stamps which cloud instance a host was
	// imported from; empty for manually managed hosts
	CloudProvider   string `gorm:"size:16;index" json:"cloud_provider"`
	CloudInstanceID string `gorm:"size:128;index" json:"cloud_instance_id"`
	CloudRegion     string `gorm:"size:64" json:"cloud_region"`
	Status        string     `gorm:"size:16;default:unknown" json:"status"` // online / offline / unknown
	LastSeen      *time.Time `json:"last_seen"`
	CreatedAt     time.Time  `json:"created_at"`

	Group  *HostGroup `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	SSHKey *SSHKey    `gorm:"foreignKey:SSHKeyID" json:"ssh_key,omitempty"`
}

type Script struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128" json:"name"`
	Description string    `json:"description"`
	Content     string    `gorm:"type:text" json:"content"`    // Linux (bash) version
	ContentPS   string    `gorm:"type:text" json:"content_ps"` // Windows (PowerShell) version; empty = no Windows variant
	Creator     string    `gorm:"size:64" json:"creator"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WebAsset is a PAM-style web application asset: URL plus vaulted credentials.
// Phase A opens a confirm card (URL/account, password masked, audited) and the
// user navigates manually; the stored design also supports one-time autofill
// tokens for a browser extension later.
type WebAsset struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128" json:"name"`
	URL         string    `gorm:"size:512" json:"url"`
	Username    string    `gorm:"size:128" json:"username"`
	Password    string    `json:"-"` // AES-GCM encrypted
	Description string    `json:"description"`
	Creator     string    `gorm:"size:64" json:"creator"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Task types
const (
	TaskCommand = "command"
	TaskScript  = "script"
	TaskFile    = "file"
	TaskRelease = "release"
	TaskCred    = "cred" // Bulk OS account creation
)

type Task struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Type       string     `gorm:"size:16;index" json:"type"`
	Operator   string     `gorm:"size:64;index" json:"operator"`
	Params     string     `gorm:"type:text" json:"params"`                     // JSON
	CronJobID  *uint      `gorm:"index" json:"cron_job_id"`                    // Cron job that triggered this task
	Status     string     `gorm:"size:16;index;default:running" json:"status"` // running / done / failed / blocked
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at"`

	Results []TaskHostResult `gorm:"foreignKey:TaskID" json:"results,omitempty"`
}

type TaskHostResult struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	TaskID     uint       `gorm:"index" json:"task_id"`
	HostID     uint       `json:"host_id"`
	HostIP     string     `gorm:"size:64" json:"host_ip"`
	HostName   string     `gorm:"size:128" json:"host_name"`
	OsUser     string     `gorm:"size:64" json:"os_user"`                // OS account used for execution
	Status     string     `gorm:"size:16;default:pending" json:"status"` // pending / running / success / failed / blocked
	ExitCode   int        `json:"exit_code"`
	Output     string     `gorm:"type:text" json:"output"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

type Application struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:128" json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`

	AppHosts []AppHost `gorm:"foreignKey:AppID" json:"app_hosts,omitempty"`
}

// Per-host deployment config for an application
type AppHost struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	AppID          uint   `gorm:"index" json:"app_id"`
	HostID         uint   `json:"host_id"`
	CredentialID   *uint  `json:"credential_id"`              // OS account used for releases (empty = host default account)
	DeployDir      string `gorm:"size:256" json:"deploy_dir"` // e.g. /app/myapp
	JarName        string `gorm:"size:256" json:"jar_name"`   // e.g. app.jar
	StopCmd        string `gorm:"size:512" json:"stop_cmd"`   // if empty, kill by process name
	StartCmd       string `gorm:"size:512" json:"start_cmd"`  // e.g. nohup java -jar ... &
	BackupDir      string `gorm:"size:256" json:"backup_dir"` // if empty, deployDir/backup is used
	HealthCheckURL string `gorm:"size:256" json:"health_check_url"`

	Host *Host `gorm:"foreignKey:HostID" json:"host,omitempty"`
}

// Release ticket
type Release struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AppID       uint      `gorm:"index" json:"app_id"`
	AppName     string    `gorm:"size:128" json:"app_name"`
	Operator    string    `gorm:"size:64" json:"operator"`
	PackageFile string    `gorm:"size:256" json:"package_file"`                // Package file name staged on the server
	PackageName string    `gorm:"size:256" json:"package_name"`                // Original file name
	Status      string    `gorm:"size:16;index;default:running" json:"status"` // running / success / failed / rollback
	CreatedAt   time.Time `json:"created_at"`

	Items []ReleaseItem `gorm:"foreignKey:ReleaseID" json:"items,omitempty"`
}

type ReleaseItem struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ReleaseID uint   `gorm:"index" json:"release_id"`
	HostID    uint   `json:"host_id"`
	HostIP    string `gorm:"size:64" json:"host_ip"`
	HostName  string `gorm:"size:128" json:"host_name"`
	Step      string `gorm:"size:16" json:"step"`   // stop / backup / upload / start / health
	Status    string `gorm:"size:16" json:"status"` // pending / running / success / failed
	Log       string `gorm:"type:text" json:"log"`
}

type DangerRule struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Pattern   string    `gorm:"size:512" json:"pattern"`
	Desc      string    `gorm:"size:256" json:"desc"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Username  string    `gorm:"size:64;index" json:"username"`
	Action    string    `gorm:"size:32;index" json:"action"` // POST / PUT / DELETE
	Resource  string    `gorm:"size:256" json:"resource"`    // Path
	Detail    string    `gorm:"type:text" json:"detail"`     // Request summary JSON
	IP        string    `gorm:"size:64" json:"ip"`
	Status    int       `json:"status"` // Response status code
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// User ↔ host group authorization (data-level permission)
type UserHostGroup struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	UserID    uint `gorm:"index" json:"user_id"`
	GroupID   uint `gorm:"index" json:"group_id"`
	CanExec   bool `json:"can_exec"`
	CanDeploy bool `json:"can_deploy"`
}

// User ↔ app release authorization
type UserApp struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `gorm:"index" json:"user_id"`
	AppID  uint `gorm:"index" json:"app_id"`
}

// User group: bulk-manages users' host access
type UserGroup struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"uniqueIndex;size:128" json:"name"`
	Description string `json:"description"`
	// When RestrictVisibility is on, group members' host lists only show hosts/host groups bound to this group (admin/ops unaffected)
	RestrictVisibility bool      `gorm:"default:false" json:"restrict_visibility"`
	CreatedAt          time.Time `json:"created_at"`
}

// User group ↔ host (direct authorization)
type UserGroupHost struct {
	ID          uint `gorm:"primaryKey" json:"id"`
	UserGroupID uint `gorm:"index" json:"user_group_id"`
	HostID      uint `gorm:"index" json:"host_id"`
}

// User group ↔ host group (whole-group authorization)
type UserGroupHostGroup struct {
	ID          uint `gorm:"primaryKey" json:"id"`
	UserGroupID uint `gorm:"index" json:"user_group_id"`
	HostGroupID uint `gorm:"index" json:"host_group_id"`
}

// User group ↔ user (group members)
type UserGroupMember struct {
	ID          uint `gorm:"primaryKey" json:"id"`
	UserGroupID uint `gorm:"index" json:"user_group_id"`
	UserID      uint `gorm:"index" json:"user_id"`
}

// Collection report: gathers info from multiple hosts by template and summarizes it
type Report struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Name       string     `gorm:"size:128" json:"name"`
	Template   string     `gorm:"size:32" json:"template"` // accounts / crontab / health / osinfo
	Operator   string     `gorm:"size:64" json:"operator"`
	HostCount  int        `json:"host_count"`
	Status     string     `gorm:"size:16;default:running" json:"status"` // running / done
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

type ReportItem struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ReportID  uint      `gorm:"index" json:"report_id"`
	HostID    uint      `json:"host_id"`
	HostName  string    `gorm:"size:128" json:"host_name"`
	HostIP    string    `gorm:"size:64" json:"host_ip"`
	Status    string    `gorm:"size:16" json:"status"` // success / failed
	Content   string    `gorm:"type:text" json:"content"`
	Error     string    `gorm:"size:255" json:"error"`
	CreatedAt time.Time `json:"created_at"`
}

// Cron job: runs a command or script on a schedule
type CronJob struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"uniqueIndex;size:128" json:"name"`
	Type         string     `gorm:"size:16;default:command" json:"type"` // command / script / report
	ReportTemplate string   `gorm:"size:64" json:"report_template"`      // type=report: preset template key
	Command      string     `gorm:"type:text" json:"command"`
	ScriptID     *uint      `json:"script_id"`
	ScriptArgs   string     `gorm:"size:512" json:"script_args"`
	HostIDs      string     `gorm:"type:text" json:"host_ids"` // JSON array
	GroupID      *uint      `json:"group_id"`
	IPs          string     `gorm:"size:512" json:"ips"`
	CredentialID *uint      `json:"credential_id"`
	CronExpr     string     `gorm:"size:64" json:"cron_expr"`
	TimeoutSec   int        `gorm:"default:300" json:"timeout_sec"`
	Concurrency  int        `gorm:"default:10" json:"concurrency"`
	Enabled      bool       `gorm:"default:true" json:"enabled"`
	NextRunAt    *time.Time `json:"next_run_at"`
	LastRunAt    *time.Time `json:"last_run_at"`
	LastTaskID   *uint      `json:"last_task_id"`
	CreatedBy    string     `gorm:"size:64" json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Host access credential: a host can have multiple OS accounts so different teams use different accounts for isolation
type HostCredential struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	HostID    uint      `gorm:"index" json:"host_id"`
	Username  string    `gorm:"size:64" json:"username"`
	AuthType  string    `gorm:"size:16;default:key" json:"auth_type"` // key / password
	SSHKeyID  *uint     `json:"ssh_key_id"`
	Password  string    `json:"-"`                    // AES-GCM encrypted
	Label     string    `gorm:"size:64" json:"label"` // Purpose: ops/app/release etc.
	IsDefault bool      `gorm:"default:false" json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	// Periodic password rotation (password-auth accounts only; LDAP/domain accounts are auto-detected and skipped)
	RotateEnabled      bool       `gorm:"default:false" json:"rotate_enabled"`
	RotateDays         int        `json:"rotate_days"` // 0 = follow the global period from system settings
	LastRotatedAt      *time.Time `json:"last_rotated_at"`
	LastRotationResult string     `gorm:"size:255" json:"last_rotation_result"`
	IsLDAP             bool       `gorm:"default:false" json:"is_ldap"` // Manually marks a domain account to exclude from rotation

	SSHKey *SSHKey `gorm:"foreignKey:SSHKeyID" json:"ssh_key,omitempty"`
}

// CredentialPasswordHistory archives every password a credential has had (AES-GCM
// encrypted), so admins can look up previous passwords after rotations. Admin-only,
// audited on every view; pruned to the most recent 24 entries per credential.
type CredentialPasswordHistory struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CredentialID uint      `gorm:"index" json:"credential_id"`
	Password     string    `json:"-"`                       // AES-GCM encrypted
	Source       string    `gorm:"size:16" json:"source"`   // created / manual / scheduled
	Operator     string    `gorm:"size:64" json:"operator"` // username or "system"
	ChangedAt    time.Time `json:"changed_at"`
}

// User group ↔ app (app team authorization: members can only see/release bound apps)
type UserGroupApp struct {
	ID          uint `gorm:"primaryKey" json:"id"`
	UserGroupID uint `gorm:"index" json:"user_group_id"`
	AppID       uint `gorm:"index" json:"app_id"`
}

// User group ↔ credential (assigns OS accounts to teams)
type UserGroupCredential struct {
	ID           uint `gorm:"primaryKey" json:"id"`
	UserGroupID  uint `gorm:"index" json:"user_group_id"`
	CredentialID uint `gorm:"index" json:"credential_id"`
}

// User group account rule: host group scope (empty = all hosts) x username; one rule covers existing and newly added hosts
type UserGroupCredRule struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	UserGroupID uint   `gorm:"index" json:"user_group_id"`
	HostGroupID *uint  `gorm:"index" json:"host_group_id"` // NULL = all hosts
	Username    string `gorm:"size:64;index" json:"username"`
}

func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// JSONParams serializes a map into a JSON string
func JSONParams(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// Application monitor (Uptime Kuma style: HTTP(s) / TCP / Ping)
type Monitor struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Name           string     `gorm:"size:128" json:"name"`
	Type           string     `gorm:"size:16" json:"type"`            // http / tcp / ping
	Target         string     `gorm:"size:256" json:"target"`         // http: full URL; tcp/ping: hostname or IP
	Port           int        `json:"port"`                           // tcp: target port
	Method         string     `gorm:"size:8" json:"method"`           // http: GET / HEAD
	AcceptedStatus string     `gorm:"size:64" json:"accepted_status"` // http: 200-299
	Keyword        string     `gorm:"size:256" json:"keyword"`        // http: keyword (optional)
	KeywordType    string     `gorm:"size:16" json:"keyword_type"`    // contain / absent
	IntervalSec    int        `json:"interval_sec"`
	TimeoutSec     int        `json:"timeout_sec"`
	Enabled        bool       `gorm:"default:true" json:"enabled"`
	DownSince      *time.Time `json:"down_since"`                // When the current outage started (cleared on recovery)
	AlertFired     bool       `json:"alert_fired"`               // Whether an alert was already sent during this outage
	LastStatus     string     `gorm:"size:8" json:"last_status"` // up / down / empty = not checked
	MonGroup       string     `gorm:"size:64" json:"mon_group"`  // display group for the app-monitor list (collapsed sections)
	OwnerGroupID   *uint      `json:"owner_group_id"`            // owning user group (department self-service monitors); NULL = infra/global
	LastRespMs     int        `json:"last_resp_ms"`
	LastError      string     `gorm:"size:255" json:"last_error"`
	LastCheckedAt  *time.Time `json:"last_checked_at"`
	NextRunAt      *time.Time `json:"-"`
	CreatedBy      string     `gorm:"size:64" json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	// HTTPS certificate lifecycle (app monitors with https targets)
	CertNotAfter  *time.Time `json:"cert_not_after"`
	CertWarnFired bool       `json:"cert_warn_fired"`
	CertCritFired bool       `json:"cert_crit_fired"`
}

// MonitorSample monitoring heartbeat sample (status + latency history, used for the heartbeat bar and uptime rate)
type MonitorSample struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	MonitorID uint      `gorm:"index" json:"monitor_id"`
	Status    string    `gorm:"size:8" json:"status"` // up / down
	RespMs    int       `json:"resp_ms"`
	Error     string    `gorm:"size:255" json:"error"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// MonitorSampleHourly pre-aggregated uptime rollup (up/down counts per monitor
// and hour): the monitor list reads availability from here instead of scanning
// millions of raw samples on every request.
type MonitorSampleHourly struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	MonitorID  uint      `gorm:"uniqueIndex:idx_mon_sample_hour,priority:1" json:"monitor_id"`
	Hour       time.Time `gorm:"uniqueIndex:idx_mon_sample_hour,priority:2" json:"hour"` // truncated to hour
	UpCount    int       `json:"up_count"`
	TotalCount int       `json:"total_count"` // up + down only (maint excluded, same as the live queries)
}

// HostMetric host resource sampling (CPU / memory / disk, collected via SSH)
type HostMetric struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	HostID      uint      `gorm:"index;index:idx_hm_host_time,priority:1" json:"host_id"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemPercent  float64   `json:"mem_percent"`
	DiskPercent float64   `json:"disk_percent"` // Max usage across all real mount points
	CollectedAt time.Time `gorm:"index;index:idx_hm_host_time,priority:2" json:"collected_at"`
}

// K8sCapacitySample K8s cluster capacity sampling (capacity planning: monthly/half-year/yearly trends and forecasts)
type K8sCapacitySample struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	ClusterID     uint      `gorm:"index" json:"cluster_id"`
	CPUCapacityM  int64     `json:"cpu_capacity_m"`
	CPUUsedM      int64     `json:"cpu_used_m"`
	MemCapacityMi int64     `json:"mem_capacity_mi"`
	MemUsedMi     int64     `json:"mem_used_mi"`
	PodReqCPUM    int64     `json:"pod_req_cpu_m"`
	PodReqMemMi   int64     `json:"pod_req_mem_mi"`
	CollectedAt   time.Time `gorm:"index" json:"collected_at"`
}

// AlertEvent alert event history (data source for monthly reports/alert stats; one row per alert sent)
type AlertEvent struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Kind        string     `gorm:"size:32;index" json:"kind"` // monitor_down / monitor_recovery / cmd_level / host_reboot / k8s_cluster_offline / k8s_cluster_recovery / k8s_node / k8s_cert / k8s_warning
	Level       string     `gorm:"size:8;index" json:"level"` // P1 / P2 / P3 / P4 / warn / info
	Target      string     `gorm:"size:255;index" json:"target"`
	Message     string     `gorm:"type:text" json:"message"`
	FiredAt     time.Time  `gorm:"index" json:"fired_at"`
	RecoveredAt *time.Time `json:"recovered_at"`
	DurationSec *int       `json:"duration_sec"`
}

// K8sPodSample pod-level capacity sampling: top 10 CPU usage per cluster (pod dimension of capacity planning)
type K8sPodSample struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ClusterID   uint      `gorm:"index:idx_kps_cluster_pod"`
	Namespace   string    `gorm:"size:63;index:idx_kps_cluster_pod"`
	Pod         string    `gorm:"size:253;index:idx_kps_cluster_pod"`
	CPUM        int64     `json:"cpu_m"`
	MemMi       int64     `json:"mem_mi"`
	CollectedAt time.Time `gorm:"index" json:"collected_at"`
}

// HostMetricHourly hourly aggregation of host metrics (backs half-year/yearly trends after raw data expires in 30 days)
type HostMetricHourly struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	HostID      uint      `gorm:"index:idx_hmh_host_bucket,unique"`
	Bucket      time.Time `gorm:"index:idx_hmh_host_bucket,unique"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemPercent  float64   `json:"mem_percent"`
	DiskPercent float64   `json:"disk_percent"`
}

// Alert notification channel (modeled on Uptime Kuma: email / webhook / WeCom / DingTalk / Feishu / Telegram)
type AlertChannel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128" json:"name"`
	Type      string    `gorm:"size:16" json:"type"`     // email / webhook / wecom / dingtalk / feishu / telegram
	Config    string    `gorm:"type:text" json:"config"` // JSON: type-specific fields
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CmdAlertState the host's current CMD alert level state (one row per host)
type CmdAlertState struct {
	HostID uint       `gorm:"primaryKey" json:"host_id"`
	Level  string     `gorm:"size:4" json:"level"` // P1-P4, empty = normal
	Since  *time.Time `json:"since"`               // When the current level started
	Fired  bool       `json:"fired"`               // Whether the alert for this level was already sent
}

// MonitorChannel monitor ↔ notification channel binding
type MonitorChannel struct {
	MonitorID uint `gorm:"primaryKey" json:"monitor_id"`
	ChannelID uint `gorm:"primaryKey" json:"channel_id"`
}

// API key (external system integration; only the SHA-256 hash is stored, the full key is shown once at creation)
type ApiKey struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"size:128" json:"name"`
	KeyID       string     `gorm:"size:32;uniqueIndex" json:"key_id"` // Public identifier
	KeyHash     string     `gorm:"size:64" json:"-"`                  // sha256(secret)
	OwnerUserID uint       `gorm:"index" json:"owner_user_id"`        // Executes as this user
	OwnerName   string     `gorm:"size:64" json:"owner_name"`
	Purpose     string     `gorm:"size:256" json:"purpose"`           // Optional usage note (what integrates via this key)
	ExpiresAt   *time.Time `json:"expires_at"`                   // Optional expiry time
	IPAllowlist string     `gorm:"size:512" json:"ip_allowlist"` // Comma-separated IP/CIDR, empty = unrestricted
	Enabled     bool       `gorm:"default:true" json:"enabled"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	LastUsedIP  string     `gorm:"size:64" json:"last_used_ip"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `gorm:"size:64" json:"created_by"`
}

// MaintenanceLog change history of maintenance windows (one row per save; active marks the current version)
type MaintenanceLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:64;index" json:"username"`
	IP        string    `gorm:"size:64" json:"ip"`
	Windows   string    `gorm:"type:text" json:"windows"` // Windows JSON
	Active    bool      `json:"active"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// K8s cluster (external system integration; credentials stored AES-GCM encrypted)
type K8sCluster struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"size:128;uniqueIndex" json:"name"`
	ApiServer   string     `gorm:"size:256" json:"api_server"`
	Support     string     `gorm:"size:128" json:"support"` // Support/contact owner
	Description string     `gorm:"size:256" json:"description"`
	CA          string     `gorm:"size:4096" json:"-"` // PEM, stored encrypted
	ClientCert  string     `gorm:"size:4096" json:"-"`
	ClientKey   string     `gorm:"size:8192" json:"-"`
	Kubeconfig  string     `gorm:"type:text" json:"-"` // Full kubeconfig, stored encrypted
	Version     string     `gorm:"size:32" json:"version"`
	NodeCount   int        `json:"node_count"`
	Status      string     `gorm:"size:16" json:"status"` // online / offline / unknown
	CertExpiry  *time.Time `json:"cert_expiry"`           // Client certificate expiry
	CAExpiry    *time.Time `json:"ca_expiry"`             // CA expiry
	LastSeen    *time.Time `json:"last_seen"`
	Warn30Sent  bool       `json:"-"` // 30-day reminder sent
	Warn7Sent   bool       `json:"-"` // 7-day reminder sent
	Enabled     bool       `gorm:"default:true" json:"enabled"`
	CreatedBy   string     `gorm:"size:64" json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// K8sClusterMember cluster member (platform role: admin / user / viewer)
type K8sClusterMember struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ClusterID uint   `gorm:"uniqueIndex:uq_k8s_cluster_user" json:"cluster_id"`
	UserID    uint   `gorm:"uniqueIndex:uq_k8s_cluster_user" json:"user_id"`
	Role      string `gorm:"size:16" json:"role"` // admin / user / viewer
	Username  string `gorm:"-" json:"username"`
}

// DbSource database ingestion source (OpenObserve builtin "DB ingestion"):
// admin-registered MySQL/MSSQL/PostgreSQL connection + query, scheduled into a stream
// CloudAccount is a CSP credential set (AK/SK or service principal) used to
// discover and import cloud instances as assets. The credential JSON is
// AES-GCM encrypted and never echoed back.
type CloudAccount struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	Provider        string     `gorm:"size:16;index" json:"provider"` // aws / azure / huawei
	Name            string     `gorm:"size:128" json:"name"`
	Credentials     string     `json:"-"`        // AES-GCM encrypted JSON (provider-specific)
	Regions         string     `gorm:"size:512" json:"regions"` // CSV of regions to scan
	TargetGroupID   *uint      `json:"target_group_id"`          // import into this host group
	TemplateID      *uint      `json:"template_id"`              // credential template: create this OS account on imported hosts
	ImportStopped   bool       `json:"import_stopped"`           // include stopped/deallocated instances
	TagGroupKey     string     `gorm:"size:32" json:"tag_group_key"` // instance tag whose value names the target host group
	SyncIntervalMin int        `json:"sync_interval_min"`        // 0 = manual only
	AutoDelete      bool       `json:"auto_delete"`              // remove hosts whose instance vanished from the cloud
	LastSyncAt      *time.Time `json:"last_sync_at"`
	LastSyncResult  string     `gorm:"size:512" json:"last_sync_result"`
	Creator         string     `gorm:"size:64" json:"creator"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// TrustedCA a root/intermediate certificate uploaded by an admin; every
// outbound TLS connection trusts the system pool plus these (see ca_trust.go)
type TrustedCA struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128" json:"name"`
	PEM         string    `gorm:"type:text" json:"-"`
	Fingerprint string    `gorm:"size:128;uniqueIndex" json:"fingerprint"`
	CreatedBy   string    `gorm:"size:64" json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type DbSource struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"size:64;uniqueIndex" json:"name"`
	DBType      string     `gorm:"size:16" json:"db_type"` // mysql / mssql / pgsql / oracle
	Host        string     `gorm:"size:128" json:"host"`
	Port        int        `json:"port"`
	Username    string     `gorm:"size:64" json:"username"`
	Password    string     `json:"-"` // AES-GCM encrypted
	Database    string     `gorm:"size:64" json:"database"`
	Query       string     `gorm:"type:text" json:"query"`
	IntervalSec int        `json:"interval_sec"`
	GroupName   string     `gorm:"size:64" json:"group_name"` // display group for the database tree
	Stream      string     `gorm:"size:100" json:"stream"`
	Enabled     bool       `json:"enabled"`
	LastRunAt   *time.Time `json:"last_run_at"`
	LastError   string     `gorm:"size:512" json:"last_error"`
	RowsPushed  int64      `json:"rows_pushed"`
	// Workbench guardrails (database workbench)
	ReadOnly   bool       `gorm:"default:false" json:"read_only"`
	TimeoutSec int        `json:"timeout_sec"` // 0 = 30
	MaxRows    int        `json:"max_rows"`    // 0 = 1000
	CreatedAt  time.Time  `json:"created_at"`
}

// DataSourceIntegration one registered collector-based data source feeding
// OpenObserve (config generated in the UI, deployed by the user)
type DataSourceIntegration struct {
	ID         int64      `gorm:"primaryKey" json:"id"`
	Name       string     `gorm:"size:128" json:"name"`
	Kind       string     `gorm:"size:64" json:"kind"`    // catalog type, e.g. nginx
	Stream     string     `gorm:"size:100" json:"stream"` // target OO stream
	Agent      string     `gorm:"size:16" json:"agent"`   // fluentbit / telegraf / otel
	HostRef    string     `gorm:"size:128" json:"host_ref"` // optional host name the collector runs on
	Notes      string     `gorm:"size:255" json:"notes"`
	Credential string     `json:"-"` // per-integration ingest credential
	Enabled    bool       `json:"enabled"`
	CreatedBy  string     `gorm:"size:64" json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
}

// DBAccount is a login under a database source. One source may carry several
// accounts with different privileges (read-only monitor account, DBA account...)
// and the workbench executes statements as the chosen account. AllowedGroups is
// a CSV of user-group IDs; empty means every workbench user may use it.
type DBAccount struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	SourceID      uint      `gorm:"index" json:"source_id"`
	Username      string    `gorm:"size:128" json:"username"`
	Password      string    `json:"-"` // AES-GCM encrypted
	Label         string    `gorm:"size:64" json:"label"`
	AllowedGroups string    `gorm:"size:512" json:"allowed_groups"` // CSV of user-group IDs; empty = all
	RotateEnabled bool      `gorm:"default:false" json:"rotate_enabled"`
	RotateDays    int       `json:"rotate_days"` // 0 = follow the global rotation period
	IsRotator     bool      `gorm:"default:false" json:"is_rotator"` // designated admin account: rotates the other accounts of this source
	LastRotatedAt *time.Time `json:"last_rotated_at"`
	LastRotationResult string `gorm:"size:255" json:"last_rotation_result"`
	CreatedAt     time.Time `json:"created_at"`
}

func (DbSource) TableName() string { return "db_sources" }
