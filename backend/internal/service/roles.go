// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"
	"strings"

	"jnexus/internal/model"
)

// RolePerm holds configurable role permissions: description + visible menus + fine-grained host permissions + OS account management
type RolePerm struct {
	Desc  string   `json:"desc"`
	Menus []string `json:"menus"`
	Host  struct {
		View   bool `json:"view"`
		Create bool `json:"create"`
		Edit   bool `json:"edit"`
		Delete bool `json:"delete"`
	} `json:"host"`
	Cred bool `json:"cred"` // OS account management (create/edit/set default; delete is always admin)

	Report bool `json:"report"` // report module (generate/view/export)

	K8sView   bool `json:"k8s_view"`   // K8S cluster view
	K8sManage bool `json:"k8s_manage"` // K8S cluster management

	// Unified capability bits (new): module → allowed operations. When empty, migrated from the legacy fields above (see legacyToPerms).
	// The new frontend role settings matrix reads/writes this field; legacy fields are kept for JSON compatibility and eventually deprecated.
	Perms map[string][]string `json:"perms,omitempty"`
}

const roleSettingsKey = "role_settings"

// allMenuKeys lists all menu keys (admin gets all by default; keep in sync when adding menu keys)
var allMenuKeys = []string{"dashboard", "shell", "hosts", "osaccounts", "paired", "exec", "tasks", "cron", "reports", "monitor", "observe", "files", "scripts", "apps", "releases", "users", "danger", "audit", "system"}

// DefaultRoleSettings returns default role settings (persisted on first use)
func DefaultRoleSettings() map[string]RolePerm {
	mk := func(desc string, menus []string, v, c, e, d, cred, report bool) RolePerm {
		r := RolePerm{Desc: desc, Menus: menus}
		r.Host.View, r.Host.Create, r.Host.Edit, r.Host.Delete = v, c, e, d
		r.Cred, r.Report = cred, report
		return r
	}
	out := map[string]RolePerm{
		model.RoleAdmin:     mk("Full access incl. users & system", append([]string{}, allMenuKeys...), true, true, true, true, true, true),
		model.RoleOps:       mk("主机、执行、文件、脚本、发布", []string{"dashboard", "hosts", "paired", "exec", "tasks", "cron", "files", "scripts", "apps", "releases", "reports", "monitor", "observe"}, true, true, true, true, true, true),
		model.RolePublisher: mk("执行与发布（需数据授权）", []string{"dashboard", "hosts", "exec", "tasks", "files", "apps", "releases"}, true, false, false, false, false, false),
		model.RoleViewer:    mk("只读查看", []string{"dashboard"}, true, false, false, false, false, false),
		model.RoleAuditor:   mk("执行记录与审计日志查看", []string{"dashboard", "tasks", "audit", "reports", "observe"}, true, false, false, false, false, true),
		model.RoleK8s:       mk("K8S 集群运维（Pod/计划任务/服务账号）", []string{"dashboard", "k8s"}, true, false, false, false, false, false),
	}
	// AI chat: granted to operational roles by default; viewer/auditor (read-only) excluded
	for role, rp := range out {
		if role == model.RoleViewer || role == model.RoleAuditor {
			continue
		}
		rp.Perms = map[string][]string{"ai": {"chat"}}
		out[role] = rp
	}
	// K8S permissions: admin view+manage; ops view only
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

// GetRoleSettings reads role settings (persists defaults when none exist)
func GetRoleSettings() map[string]RolePerm {
	var sc model.SystemConfig
	if err := model.DB.Where("key = ?", roleSettingsKey).First(&sc).Error; err != nil || sc.Value == "" {
		def := DefaultRoleSettings()
		SetRoleSettings(def)
		return expandMonitorPerms(withAIGrants(def))
	}
	out := map[string]RolePerm{}
	if err := json.Unmarshal([]byte(sc.Value), &out); err != nil {
		return expandMonitorPerms(withAIGrants(DefaultRoleSettings()))
	}
	// fill in defaults for newly added roles; backfill the cred field with defaults for stored configs missing it (avoiding silent permission loss on old data)
	def := DefaultRoleSettings()
	legacy := !strings.Contains(sc.Value, "\"cred\"")
	for role, d := range def {
		if _, ok := out[role]; !ok {
			out[role] = d
			continue
		}
		rp := out[role] // struct fetched from the map must be copied before modifying
		if legacy || !strings.Contains(sc.Value, "\"k8s_view\"") {
			// K8S permissions are a later-added field: backfill per role defaults (admin manage, ops view)
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
		// Unified capability bit backfill: migrate once from legacy fields when stored config has no Perms
		if len(rp.Perms) == 0 {
			rp.Perms = legacyToPerms(role, rp)
		}
		// AI chat default grants: ops/publisher/k8s get ai:chat (viewer/auditor excluded).
		// Runs only while the stored config has no explicit "ai" entry; once the role
		// matrix is saved, the stored Perms control completely.
		if !strings.Contains(sc.Value, `"ai"`) {
			switch role {
			case model.RoleOps, model.RolePublisher, model.RoleK8s:
				rp.Perms["ai"] = []string{"chat"}
			case model.RoleViewer, model.RoleAuditor:
				rp.Perms["ai"] = []string{}
			}
		}
		// auto-add new menus to admin/ops/auditor (admin always sees everything)
		if role == model.RoleAdmin || role == model.RoleOps || role == model.RoleAuditor {
			for _, nm := range []string{"cron", "osaccounts", "reports", "monitor", "k8s", "observe"} {
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
		// complete the admin menu set to the full list (admin is not restricted by menus; kept only for display consistency)
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
	return expandMonitorPerms(withAIGrants(out))
}

// withAIGrants applies in-memory-only default AI chat grants: ops/publisher/k8s
// get ai:chat; viewer/auditor stay excluded. Runs while the stored roles config
// has no explicit "ai" entry — once the role matrix is saved, the stored Perms
// control completely. Never persisted, so the legacy permission migration
// (which keys off empty Perms) is unaffected.
func withAIGrants(out map[string]RolePerm) map[string]RolePerm {
	for role, rp := range out {
		if _, has := rp.Perms["ai"]; has {
			continue
		}
		if rp.Perms == nil {
			rp.Perms = map[string][]string{}
		}
		switch role {
		case model.RoleOps, model.RolePublisher, model.RoleK8s:
			rp.Perms["ai"] = []string{"chat"}
		default:
			rp.Perms["ai"] = []string{}
		}
		out[role] = rp
	}
	return out
}

// HasK8sPerm checks K8S permissions (kind: view / manage; delegates to unified capability bits)
func HasK8sPerm(role, kind string) bool {
	return HasCap(role, "k8s", kind)
}

// HasCredPerm reports whether the role has OS account management permission (delegates to unified capability bits)
func HasCredPerm(role string) bool {
	return HasCap(role, "credentials", "manage")
}

// HasReportPerm reports whether the role can use the report module (delegates to unified capability bits)
func HasReportPerm(role string) bool {
	return HasCap(role, "reports", "view")
}

// SetRoleSettings saves role settings
func SetRoleSettings(m map[string]RolePerm) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return model.DB.Save(&model.SystemConfig{Key: roleSettingsKey, Value: string(b)}).Error
}

// HasHostPerm checks whether the role has the host operation permission (admin always passes)
func HasHostPerm(role, action string) bool {
	if role == model.RoleAdmin {
		return true
	}
	return HasCap(role, "hosts", action)
}

// expandMonitorPerms converts legacy monitor entries ("view" / "manage") in
// stored configs to the split actions (view_host/view_app/view_sec/manage)
// in memory. Once the role matrix is saved, the stored actions control as-is.
func expandMonitorPerms(out map[string]RolePerm) map[string]RolePerm {
	for role, rp := range out {
		acts := rp.Perms["monitor"]
		if len(acts) == 0 {
			continue
		}
		legacy := false
		set := map[string]bool{}
		for _, a := range acts {
			switch a {
			case "view":
				legacy = true
				set["view_host"], set["view_app"], set["view_sec"] = true, true, true
			case "manage":
				legacy = true
				set["view_host"], set["view_app"], set["view_sec"], set["manage"] = true, true, true, true
			default:
				set[a] = true
			}
		}
		if !legacy {
			continue
		}
		next := []string{}
		for _, k := range []string{"view_host", "view_app", "view_sec", "manage"} {
			if set[k] {
				next = append(next, k)
			}
		}
		rp.Perms["monitor"] = next
		out[role] = rp
	}
	return out
}
