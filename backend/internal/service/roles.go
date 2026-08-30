// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"

	"autoops/internal/model"
)

// RolePerm 角色可配置权限：描述 + 可见菜单 + 主机细粒度权限
type RolePerm struct {
	Desc  string   `json:"desc"`
	Menus []string `json:"menus"`
	Host  struct {
		View   bool `json:"view"`
		Create bool `json:"create"`
		Edit   bool `json:"edit"`
		Delete bool `json:"delete"`
	} `json:"host"`
}

const roleSettingsKey = "role_settings"

// DefaultRoleSettings 角色默认配置（首次使用时写入）
func DefaultRoleSettings() map[string]RolePerm {
	allMenus := []string{"dashboard", "hosts", "exec", "tasks", "files", "scripts", "apps", "releases", "users", "danger", "audit", "system"}
	mk := func(desc string, menus []string, v, c, e, d bool) RolePerm {
		r := RolePerm{Desc: desc, Menus: menus}
		r.Host.View, r.Host.Create, r.Host.Edit, r.Host.Delete = v, c, e, d
		return r
	}
	return map[string]RolePerm{
		model.RoleAdmin:     mk("全部权限，含用户/系统管理", allMenus, true, true, true, true),
		model.RoleOps:       mk("主机、执行、文件、脚本、发布", []string{"dashboard", "hosts", "exec", "tasks", "files", "scripts", "apps", "releases"}, true, true, true, true),
		model.RolePublisher: mk("执行与发布（需数据授权）", []string{"dashboard", "hosts", "exec", "tasks", "files", "apps", "releases"}, true, false, false, false),
		model.RoleViewer:    mk("只读查看", []string{"dashboard", "hosts"}, true, false, false, false),
		model.RoleAuditor:   mk("执行记录与审计日志查看", []string{"dashboard", "tasks", "audit"}, true, false, false, false),
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
	// 补齐新增角色的默认值
	def := DefaultRoleSettings()
	for role, d := range def {
		if _, ok := out[role]; !ok {
			out[role] = d
		}
	}
	return out
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
