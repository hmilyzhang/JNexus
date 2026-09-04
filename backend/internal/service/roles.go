// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"
	"strings"

	"autoops/internal/model"
)

// RolePerm 角色可配置权限：描述 + 可见菜单 + 主机细粒度权限 + OS 账号管理
type RolePerm struct {
	Desc  string   `json:"desc"`
	Menus []string `json:"menus"`
	Host  struct {
		View   bool `json:"view"`
		Create bool `json:"create"`
		Edit   bool `json:"edit"`
		Delete bool `json:"delete"`
	} `json:"host"`
	Cred bool `json:"cred"` // OS 账号管理（新增/编辑/设默认；删除恒为 admin）

	Report bool `json:"report"` // 报告模块（生成/查看/导出）
}

const roleSettingsKey = "role_settings"

// allMenuKeys 全部菜单键（admin 默认全量；新增菜单键时须同步）
var allMenuKeys = []string{"dashboard", "shell", "hosts", "osaccounts", "paired", "exec", "tasks", "cron", "reports", "monitor", "files", "scripts", "apps", "releases", "users", "danger", "audit", "system"}

// DefaultRoleSettings 角色默认配置（首次使用时写入）
func DefaultRoleSettings() map[string]RolePerm {
	mk := func(desc string, menus []string, v, c, e, d, cred, report bool) RolePerm {
		r := RolePerm{Desc: desc, Menus: menus}
		r.Host.View, r.Host.Create, r.Host.Edit, r.Host.Delete = v, c, e, d
		r.Cred, r.Report = cred, report
		return r
	}
	return map[string]RolePerm{
		model.RoleAdmin:     mk("Full access incl. users & system", append([]string{}, allMenuKeys...), true, true, true, true, true, true),
		model.RoleOps:       mk("主机、执行、文件、脚本、发布", []string{"dashboard", "hosts", "paired", "exec", "tasks", "cron", "files", "scripts", "apps", "releases", "reports"}, true, true, true, true, true, true),
		model.RolePublisher: mk("执行与发布（需数据授权）", []string{"dashboard", "hosts", "exec", "tasks", "files", "apps", "releases"}, true, false, false, false, false, false),
		model.RoleViewer:    mk("只读查看", []string{"dashboard", "hosts"}, true, false, false, false, false, false),
		model.RoleAuditor:   mk("执行记录与审计日志查看", []string{"dashboard", "tasks", "audit", "reports"}, true, false, false, false, false, true),
	}
}

// GetRoleSettings 读取角色配置（无则落库默认值）
func GetRoleSettings() map[string]RolePerm {
	var sc model.SystemConfig
	if err := model.DB.Where("key = ?", roleSettingsKey).First(&sc).Error; err != nil || sc.Value == "" {
		def := DefaultRoleSettings()
		SetRoleSettings(def)
		return def
	}
	out := map[string]RolePerm{}
	if err := json.Unmarshal([]byte(sc.Value), &out); err != nil {
		return DefaultRoleSettings()
	}
	// 补齐新增角色的默认值；存量配置缺 cred 字段时按默认值回填（避免旧数据静默失权）
	def := DefaultRoleSettings()
	legacy := !strings.Contains(sc.Value, "\"cred\"")
	for role, d := range def {
		if _, ok := out[role]; !ok {
			out[role] = d
			continue
		}
		rp := out[role] // map 取出的结构体需复制后修改
		if legacy {
			rp.Cred = d.Cred
			rp.Report = d.Report
		}
		// 新增菜单自动补进 admin/ops/auditor（admin 恒见全部）
		if role == model.RoleAdmin || role == model.RoleOps || role == model.RoleAuditor {
			for _, nm := range []string{"cron", "osaccounts", "reports", "monitor"} {
				has := false
				for _, m := range rp.Menus {
					if m == nm {
						has = true
						break
					}
				}
				if !has {
					rp.Menus = append(rp.Menus, nm)
				}
			}
		}
		// admin 菜单集合补齐为全量（admin 不受菜单限制，仅保持显示一致）
		if role == model.RoleAdmin {
			for _, nm := range allMenuKeys {
				has := false
				for _, m := range rp.Menus {
					if m == nm {
						has = true
						break
					}
				}
				if !has {
					rp.Menus = append(rp.Menus, nm)
				}
			}
		}
		out[role] = rp
	}
	return out
}

// HasCredPerm 角色是否拥有 OS 账号管理权限（admin 恒通过）
func HasCredPerm(role string) bool {
	if role == model.RoleAdmin {
		return true
	}
	return GetRoleSettings()[role].Cred
}

// HasReportPerm 角色是否可使用报告模块（admin 恒通过）
func HasReportPerm(role string) bool {
	if role == model.RoleAdmin {
		return true
	}
	return GetRoleSettings()[role].Report
}

// SetRoleSettings 保存角色配置
func SetRoleSettings(m map[string]RolePerm) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return model.DB.Save(&model.SystemConfig{Key: roleSettingsKey, Value: string(b)}).Error
}

// HasHostPerm 检查角色是否拥有主机操作权限（admin 恒通过）
func HasHostPerm(role, action string) bool {
	if role == model.RoleAdmin {
		return true
	}
	settings := GetRoleSettings()
	r, ok := settings[role]
	if !ok {
		return false
	}
	switch action {
	case "view":
		return r.Host.View
	case "create":
		return r.Host.Create
	case "edit":
		return r.Host.Edit
	case "delete":
		return r.Host.Delete
	}
	return false
}
