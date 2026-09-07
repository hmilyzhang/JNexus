// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"
	"strings"

	"jnexus/internal/model"
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

	K8sView   bool `json:"k8s_view"`   // K8S 集群查看
	K8sManage bool `json:"k8s_manage"` // K8S 集群管理

	// 统一能力位（新）：模块 → 允许的操作。为空时从上面的旧字段迁移（见 legacyToPerms）。
	// 新前端角色设置矩阵读写此字段；旧字段保留用于 JSON 兼容，最终废弃。
	Perms map[string][]string `json:"perms,omitempty"`
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
	out := map[string]RolePerm{
		model.RoleAdmin:     mk("Full access incl. users & system", append([]string{}, allMenuKeys...), true, true, true, true, true, true),
		model.RoleOps:       mk("主机、执行、文件、脚本、发布", []string{"dashboard", "hosts", "paired", "exec", "tasks", "cron", "files", "scripts", "apps", "releases", "reports"}, true, true, true, true, true, true),
		model.RolePublisher: mk("执行与发布（需数据授权）", []string{"dashboard", "hosts", "exec", "tasks", "files", "apps", "releases"}, true, false, false, false, false, false),
		model.RoleViewer:    mk("只读查看", []string{"dashboard"}, true, false, false, false, false, false),
		model.RoleAuditor:   mk("执行记录与审计日志查看", []string{"dashboard", "tasks", "audit", "reports"}, true, false, false, false, false, true),
		model.RoleK8s:       mk("K8S 集群运维（Pod/计划任务/服务账号）", []string{"dashboard", "k8s"}, true, false, false, false, false, false),
	}
	// K8S 权限：admin 查看+管理；ops 查看
	a := out[model.RoleAdmin]
	a.K8sView, a.K8sManage = true, true
	out[model.RoleAdmin] = a
	o := out[model.RoleOps]
	o.K8sView, o.K8sManage = true, false
	out[model.RoleOps] = o
	k := out[model.RoleK8s]
	k.K8sView, k.K8sManage = true, false
	out[model.RoleK8s] = k
	return out
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
		if legacy || !strings.Contains(sc.Value, "\"k8s_view\"") {
			// K8S 权限为后加字段：按角色默认回填（admin 管理，ops 查看）
			switch role {
			case model.RoleAdmin:
				rp.K8sView, rp.K8sManage = true, true
			case model.RoleOps:
				rp.K8sView, rp.K8sManage = true, false
			case model.RoleK8s:
				rp.K8sView, rp.K8sManage = true, false
			}
		}
		if legacy {
			rp.Cred = d.Cred
			rp.Report = d.Report
		}
		// 统一能力位回填：存量配置无 Perms 时从旧字段迁移一次
		if len(rp.Perms) == 0 {
			rp.Perms = legacyToPerms(role, rp)
		}
		// 新增菜单自动补进 admin/ops/auditor（admin 恒见全部）
		if role == model.RoleAdmin || role == model.RoleOps || role == model.RoleAuditor {
			for _, nm := range []string{"cron", "osaccounts", "reports", "monitor", "k8s"} {
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

// HasK8sPerm K8S 权限检查（kind: view / manage；委托统一能力位）
func HasK8sPerm(role, kind string) bool {
	return HasCap(role, "k8s", kind)
}

// HasCredPerm 角色是否拥有 OS 账号管理权限（委托统一能力位）
func HasCredPerm(role string) bool {
	return HasCap(role, "credentials", "manage")
}

// HasReportPerm 角色是否可使用报告模块（委托统一能力位）
func HasReportPerm(role string) bool {
	return HasCap(role, "reports", "view")
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
	return HasCap(role, "hosts", action)
}
