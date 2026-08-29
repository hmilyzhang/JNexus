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
)

type User struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Username    string    `gorm:"uniqueIndex;size:64" json:"username"`
	Password    string    `json:"-"`
	Role        string    `gorm:"size:32;index" json:"role"`
	Status      int       `gorm:"default:1" json:"status"` // 1 启用 0 禁用
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type HostGroup struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"uniqueIndex;size:128" json:"name"`
	Description string `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type SSHKey struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;size:128" json:"name"`
	PublicKey string    `json:"public_key"`
	// AES-GCM 加密后的私钥
	PrivateKey string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
}

type Host struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128" json:"name"`
	IP        string    `gorm:"size:64;index" json:"ip"`
	Port      int       `gorm:"default:22" json:"port"`
	Username  string    `gorm:"size:64" json:"username"`
	AuthType  string    `gorm:"size:16;default:key" json:"auth_type"` // key / password
	SSHKeyID  *uint     `json:"ssh_key_id"`
	Password  string    `json:"-"` // AES-GCM 加密
	GroupID   *uint     `gorm:"index" json:"group_id"`
	Status    string    `gorm:"size:16;default:unknown" json:"status"` // online / offline / unknown
	LastSeen  *time.Time `json:"last_seen"`
	CreatedAt time.Time  `json:"created_at"`

	Group *HostGroup `gorm:"foreignKey:GroupID" json:"group,omitempty"`
	SSHKey *SSHKey   `gorm:"foreignKey:SSHKeyID" json:"ssh_key,omitempty"`
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
)

type Task struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Type      string         `gorm:"size:16;index" json:"type"`
	Operator  string         `gorm:"size:64;index" json:"operator"`
	Params    string         `gorm:"type:text" json:"params"` // JSON
	Status    string         `gorm:"size:16;index;default:running" json:"status"` // running / done / failed / blocked
	CreatedAt time.Time      `json:"created_at"`
	FinishedAt *time.Time    `json:"finished_at"`

	Results []TaskHostResult `gorm:"foreignKey:TaskID" json:"results,omitempty"`
}

type TaskHostResult struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    uint      `gorm:"index" json:"task_id"`
	HostID    uint      `json:"host_id"`
	HostIP    string    `gorm:"size:64" json:"host_ip"`
	HostName  string    `gorm:"size:128" json:"host_name"`
	Status    string    `gorm:"size:16;default:pending" json:"status"` // pending / running / success / failed / blocked
	ExitCode  int       `json:"exit_code"`
	Output    string    `gorm:"type:text" json:"output"`
	CreatedAt time.Time `json:"created_at"`
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
	DeployDir      string `gorm:"size:256" json:"deploy_dir"`      // 如 /app/myapp
	JarName        string `gorm:"size:256" json:"jar_name"`        // 如 app.jar
	StopCmd        string `gorm:"size:512" json:"stop_cmd"`        // 为空则按进程名 kill
	StartCmd       string `gorm:"size:512" json:"start_cmd"`       // 如 nohup java -jar ... &
	BackupDir      string `gorm:"size:256" json:"backup_dir"`      // 为空用 deployDir/backup
	HealthCheckURL string `gorm:"size:256" json:"health_check_url"`

	Host *Host `gorm:"foreignKey:HostID" json:"host,omitempty"`
}

// 发布单
type Release struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AppID     uint      `gorm:"index" json:"app_id"`
	AppName   string    `gorm:"size:128" json:"app_name"`
	Operator  string    `gorm:"size:64" json:"operator"`
	PackageFile string  `gorm:"size:256" json:"package_file"` // 服务端暂存的包文件名
	PackageName string  `gorm:"size:256" json:"package_name"` // 原始文件名
	Status    string    `gorm:"size:16;index;default:running" json:"status"` // running / success / failed / rollback
	CreatedAt time.Time `json:"created_at"`

	Items []ReleaseItem `gorm:"foreignKey:ReleaseID" json:"items,omitempty"`
}

type ReleaseItem struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	ReleaseID  uint   `gorm:"index" json:"release_id"`
	HostID     uint   `json:"host_id"`
	HostIP     string `gorm:"size:64" json:"host_ip"`
	HostName   string `gorm:"size:128" json:"host_name"`
	Step       string `gorm:"size:16" json:"step"`     // stop / backup / upload / start / health
	Status     string `gorm:"size:16" json:"status"`   // pending / running / success / failed
	Log        string `gorm:"type:text" json:"log"`
}

type DangerRule struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Pattern   string    `gorm:"size:512" json:"pattern"`
	Desc      string    `gorm:"size:256" json:"desc"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditLog struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"index" json:"user_id"`
	Username  string         `gorm:"size:64;index" json:"username"`
	Action    string         `gorm:"size:32;index" json:"action"` // POST / PUT / DELETE
	Resource  string         `gorm:"size:256" json:"resource"`    // 路径
	Detail    string         `gorm:"type:text" json:"detail"`     // 请求摘要 JSON
	IP        string         `gorm:"size:64" json:"ip"`
	Status    int            `json:"status"` // 响应码
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
}

// 用户 ↔ 主机分组授权（数据级权限）
type UserHostGroup struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	UserID   uint `gorm:"index" json:"user_id"`
	GroupID  uint `gorm:"index" json:"group_id"`
	CanExec  bool `json:"can_exec"`
	CanDeploy bool `json:"can_deploy"`
}

// 用户 ↔ 应用发布授权
type UserApp struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	UserID   uint `gorm:"index" json:"user_id"`
	AppID    uint `gorm:"index" json:"app_id"`
}

func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// JSONParams 把 map 序列化为 JSON 字符串
func JSONParams(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}
