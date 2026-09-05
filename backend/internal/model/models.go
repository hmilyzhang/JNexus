// AutoOps 运维平台 — By JJ Zhang, Version 1.0

package model

import (
	"encoding/json"
	"time"
)

// 角色
const (
	RoleAdmin     = "admin"     // 管理员
	RoleOps       = "ops"       // 运维
	RolePublisher = "publisher" // 发布员
	RoleViewer    = "viewer"    // 只读
	RoleAuditor   = "auditor"   // 审计员：可查看执行记录与审计日志
)

type User struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Username    string     `gorm:"uniqueIndex;size:64" json:"username"`
	Password    string     `json:"-"`
	Role        string     `gorm:"size:32;index" json:"role"`
	AuthSource  string     `gorm:"size:16;default:local" json:"auth_source"` // local / ldap
	Email       string     `gorm:"size:128" json:"email"`
	Status      int        `gorm:"default:1" json:"status"` // 1 启用 0 禁用
	MFAEnabled  bool       `gorm:"default:false" json:"mfa_enabled"`
	MFASecret   string     `gorm:"size:256" json:"-"` // TOTP 密钥，AES-GCM 加密存储
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `gorm:"size:64" json:"created_by"` // 创建人（LDAP 自动建号记为 LDAP）
	UpdatedBy   string     `gorm:"size:64" json:"updated_by"` // 最近修改人
	UpdatedAt   time.Time  `json:"updated_at"`
	DisabledAt  *time.Time `json:"disabled_at"`                // 禁用时间（启用后清空）
	DisabledBy  string     `gorm:"size:64" json:"disabled_by"` // 禁用操作人
}

// SystemConfig 系统配置（key-value，值可为 JSON 文本）
type SystemConfig struct {
	Key   string `gorm:"primaryKey;size:64" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

type HostGroup struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:128" json:"name"`
	ParentID    *uint     `gorm:"index" json:"parent_id"` // 上级分组（多级树）
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type SSHKey struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Name      string `gorm:"uniqueIndex;size:128" json:"name"`
	PublicKey string `json:"public_key"`
	// AES-GCM 加密后的私钥
	PrivateKey string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
}

type Host struct {
	LastBootID string     `gorm:"size:64" json:"-"` // 上次采集到的 boot_id（用于检测系统重启）
	ID         uint       `gorm:"primaryKey" json:"id"`
	Name       string     `gorm:"size:128" json:"name"`
	IP         string     `gorm:"size:64;index" json:"ip"`
	Port       int        `gorm:"default:22" json:"port"`
	Username   string     `gorm:"size:64" json:"username"`
	AuthType   string     `gorm:"size:16;default:key" json:"auth_type"` // key / password
	SSHKeyID   *uint      `json:"ssh_key_id"`
	Password   string     `json:"-"` // AES-GCM 加密
	GroupID    *uint      `gorm:"index" json:"group_id"`
	Status     string     `gorm:"size:16;default:unknown" json:"status"` // online / offline / unknown
	LastSeen   *time.Time `json:"last_seen"`
	CreatedAt  time.Time  `json:"created_at"`

	Group  *HostGroup `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	SSHKey *SSHKey    `gorm:"foreignKey:SSHKeyID" json:"ssh_key,omitempty"`
}

type Script struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128" json:"name"`
	Description string    `json:"description"`
	Content     string    `gorm:"type:text" json:"content"`
	Creator     string    `gorm:"size:64" json:"creator"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// 任务类型
const (
	TaskCommand = "command"
	TaskScript  = "script"
	TaskFile    = "file"
	TaskRelease = "release"
	TaskCred    = "cred" // 批量添加 OS 账号
)

type Task struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Type       string     `gorm:"size:16;index" json:"type"`
	Operator   string     `gorm:"size:64;index" json:"operator"`
	Params     string     `gorm:"type:text" json:"params"`                     // JSON
	CronJobID  *uint      `gorm:"index" json:"cron_job_id"`                    // 计划任务触发来源
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
	OsUser     string     `gorm:"size:64" json:"os_user"`                // 执行使用的 OS 账号
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

// 应用在某台主机上的部署配置
type AppHost struct {
	ID             uint   `gorm:"primaryKey" json:"id"`
	AppID          uint   `gorm:"index" json:"app_id"`
	HostID         uint   `json:"host_id"`
	CredentialID   *uint  `json:"credential_id"`              // 发布使用的 OS 账号（空=主机默认账号）
	DeployDir      string `gorm:"size:256" json:"deploy_dir"` // 如 /app/myapp
	JarName        string `gorm:"size:256" json:"jar_name"`   // 如 app.jar
	StopCmd        string `gorm:"size:512" json:"stop_cmd"`   // 为空则按进程名 kill
	StartCmd       string `gorm:"size:512" json:"start_cmd"`  // 如 nohup java -jar ... &
	BackupDir      string `gorm:"size:256" json:"backup_dir"` // 为空用 deployDir/backup
	HealthCheckURL string `gorm:"size:256" json:"health_check_url"`

	Host *Host `gorm:"foreignKey:HostID" json:"host,omitempty"`
}

// 发布单
type Release struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AppID       uint      `gorm:"index" json:"app_id"`
	AppName     string    `gorm:"size:128" json:"app_name"`
	Operator    string    `gorm:"size:64" json:"operator"`
	PackageFile string    `gorm:"size:256" json:"package_file"`                // 服务端暂存的包文件名
	PackageName string    `gorm:"size:256" json:"package_name"`                // 原始文件名
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
	Resource  string    `gorm:"size:256" json:"resource"`    // 路径
	Detail    string    `gorm:"type:text" json:"detail"`     // 请求摘要 JSON
	IP        string    `gorm:"size:64" json:"ip"`
	Status    int       `json:"status"` // 响应码
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// 用户 ↔ 主机分组授权（数据级权限）
type UserHostGroup struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	UserID    uint `gorm:"index" json:"user_id"`
	GroupID   uint `gorm:"index" json:"group_id"`
	CanExec   bool `json:"can_exec"`
	CanDeploy bool `json:"can_deploy"`
}

// 用户 ↔ 应用发布授权
type UserApp struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `gorm:"index" json:"user_id"`
	AppID  uint `gorm:"index" json:"app_id"`
}

// 用户组：批量管理用户的主机访问权限
type UserGroup struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:128" json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// 用户组 ↔ 主机（直接授权）
type UserGroupHost struct {
	ID          uint `gorm:"primaryKey" json:"id"`
	UserGroupID uint `gorm:"index" json:"user_group_id"`
	HostID      uint `gorm:"index" json:"host_id"`
}

// 用户组 ↔ 主机分组（整组授权）
type UserGroupHostGroup struct {
	ID          uint `gorm:"primaryKey" json:"id"`
	UserGroupID uint `gorm:"index" json:"user_group_id"`
	HostGroupID uint `gorm:"index" json:"host_group_id"`
}

// 用户组 ↔ 用户（组成员）
type UserGroupMember struct {
	ID          uint `gorm:"primaryKey" json:"id"`
	UserGroupID uint `gorm:"index" json:"user_group_id"`
	UserID      uint `gorm:"index" json:"user_id"`
}

// 采集报告：按模板在多台主机上收集信息并汇总
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

// 计划任务：定时执行命令或脚本
type CronJob struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"uniqueIndex;size:128" json:"name"`
	Type         string     `gorm:"size:16;default:command" json:"type"` // command / script
	Command      string     `gorm:"type:text" json:"command"`
	ScriptID     *uint      `json:"script_id"`
	ScriptArgs   string     `gorm:"size:512" json:"script_args"`
	HostIDs      string     `gorm:"type:text" json:"host_ids"` // JSON 数组
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

// 主机访问凭据：一台主机可挂多个 OS 账号，不同团队使用不同账号实现账号隔离
type HostCredential struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	HostID    uint      `gorm:"index" json:"host_id"`
	Username  string    `gorm:"size:64" json:"username"`
	AuthType  string    `gorm:"size:16;default:key" json:"auth_type"` // key / password
	SSHKeyID  *uint     `json:"ssh_key_id"`
	Password  string    `json:"-"`                    // AES-GCM 加密
	Label     string    `gorm:"size:64" json:"label"` // 用途：运维/应用/发布等
	IsDefault bool      `gorm:"default:false" json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	// 密码定期轮换（仅密码认证的账号；LDAP/域账号自动检测跳过）
	RotateEnabled      bool       `gorm:"default:false" json:"rotate_enabled"`
	RotateDays         int        `json:"rotate_days"` // 0 = 跟随系统设置的全局周期
	LastRotatedAt      *time.Time `json:"last_rotated_at"`
	LastRotationResult string     `gorm:"size:255" json:"last_rotation_result"`
	IsLDAP             bool       `gorm:"default:false" json:"is_ldap"` // 手动标记域账号，排除轮换

	SSHKey *SSHKey `gorm:"foreignKey:SSHKeyID" json:"ssh_key,omitempty"`
}

// 用户组 ↔ 凭据（把 OS 账号分配给团队）
type UserGroupCredential struct {
	ID           uint `gorm:"primaryKey" json:"id"`
	UserGroupID  uint `gorm:"index" json:"user_group_id"`
	CredentialID uint `gorm:"index" json:"credential_id"`
}

// 用户组账号规则：主机分组范围（空=全部主机）× 账号名，一次规则覆盖存量与新增主机
type UserGroupCredRule struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	UserGroupID uint   `gorm:"index" json:"user_group_id"`
	HostGroupID *uint  `gorm:"index" json:"host_group_id"` // NULL=全部主机
	Username    string `gorm:"size:64;index" json:"username"`
}

func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// JSONParams 把 map 序列化为 JSON 字符串
func JSONParams(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// 应用监控项（Uptime Kuma 风格：HTTP(s) / TCP / Ping）
type Monitor struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Name           string     `gorm:"size:128" json:"name"`
	Type           string     `gorm:"size:16" json:"type"`            // http / tcp / ping
	Target         string     `gorm:"size:256" json:"target"`         // http: 完整 URL；tcp/ping: 主机名或 IP
	Port           int        `json:"port"`                           // tcp: 目标端口
	Method         string     `gorm:"size:8" json:"method"`           // http: GET / HEAD
	AcceptedStatus string     `gorm:"size:64" json:"accepted_status"` // http: 200-299
	Keyword        string     `gorm:"size:256" json:"keyword"`        // http: 关键字（可选）
	KeywordType    string     `gorm:"size:16" json:"keyword_type"`    // contain / absent
	IntervalSec    int        `json:"interval_sec"`
	TimeoutSec     int        `json:"timeout_sec"`
	Enabled        bool       `gorm:"default:true" json:"enabled"`
	DownSince      *time.Time `json:"down_since"`                // 当前故障开始时间（恢复后清空）
	AlertFired     bool       `json:"alert_fired"`               // 本次故障周期内是否已发送告警
	LastStatus     string     `gorm:"size:8" json:"last_status"` // up / down / 空=未检查
	LastRespMs     int        `json:"last_resp_ms"`
	LastError      string     `gorm:"size:255" json:"last_error"`
	LastCheckedAt  *time.Time `json:"last_checked_at"`
	NextRunAt      *time.Time `json:"-"`
	CreatedBy      string     `gorm:"size:64" json:"created_by"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// MonitorSample 监控心跳样本（状态 + 耗时历史，用于心跳条与可用率）
type MonitorSample struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	MonitorID uint      `gorm:"index" json:"monitor_id"`
	Status    string    `gorm:"size:8" json:"status"` // up / down
	RespMs    int       `json:"resp_ms"`
	Error     string    `gorm:"size:255" json:"error"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// HostMetric 主机基础资源采样（CPU / 内存 / 磁盘，经 SSH 采集）
type HostMetric struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	HostID      uint      `gorm:"index" json:"host_id"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemPercent  float64   `json:"mem_percent"`
	DiskPercent float64   `json:"disk_percent"` // 所有真实挂载点中最大使用率
	CollectedAt time.Time `gorm:"index" json:"collected_at"`
}

// 告警通知通道（参考 Uptime Kuma：邮件 / Webhook / 企业微信 / 钉钉 / 飞书 / Telegram）
type AlertChannel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128" json:"name"`
	Type      string    `gorm:"size:16" json:"type"`     // email / webhook / wecom / dingtalk / feishu / telegram
	Config    string    `gorm:"type:text" json:"config"` // JSON：各类型自己的字段
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CmdAlertState 主机当前所处的 CMD 告警级别状态（每主机一行）
type CmdAlertState struct {
	HostID uint       `gorm:"primaryKey" json:"host_id"`
	Level  string     `gorm:"size:4" json:"level"` // P1-P4，空=正常
	Since  *time.Time `json:"since"`               // 当前级别起始时间
	Fired  bool       `json:"fired"`               // 该级别告警是否已发送
}

// MonitorChannel 监控项 ↔ 通知通道 绑定
type MonitorChannel struct {
	MonitorID uint `gorm:"primaryKey" json:"monitor_id"`
	ChannelID uint `gorm:"primaryKey" json:"channel_id"`
}

// API 密钥（外部系统集成；仅存 SHA-256 哈希，完整密钥仅创建时展示一次）
type ApiKey struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Name        string     `gorm:"size:128" json:"name"`
	KeyID       string     `gorm:"size:32;uniqueIndex" json:"key_id"` // 公开标识
	KeyHash     string     `gorm:"size:64" json:"-"`                  // sha256(secret)
	OwnerUserID uint       `gorm:"index" json:"owner_user_id"`        // 以该用户身份执行
	OwnerName   string     `gorm:"size:64" json:"owner_name"`
	ExpiresAt   *time.Time `json:"expires_at"`                   // 可选过期时间
	IPAllowlist string     `gorm:"size:512" json:"ip_allowlist"` // 逗号分隔 IP/CIDR，空=不限
	Enabled     bool       `gorm:"default:true" json:"enabled"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	LastUsedIP  string     `gorm:"size:64" json:"last_used_ip"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `gorm:"size:64" json:"created_by"`
}

// MaintenanceLog 维护窗口变更留痕（每次保存一条；active 标记当前生效版本）
type MaintenanceLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:64;index" json:"username"`
	IP        string    `gorm:"size:64" json:"ip"`
	Windows   string    `gorm:"type:text" json:"windows"` // 窗口 JSON
	Active    bool      `json:"active"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
