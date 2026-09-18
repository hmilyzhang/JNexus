// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"jnexus/internal/model"
)

// Unified capability-bit permission model:
// each module registers its supported actions in ModuleCapabilities, routes check via RequireCap(module, action),
// and role config expresses grants as Perms[module] = [action...]. Menu keys share the module keys, keeping menus and APIs consistent.

// CapModule module/action declaration for the frontend role settings matrix
type CapModule struct {
	Key     string   `json:"key"`
	Actions []string `json:"actions"`
}

// ModuleCapabilities registry of all module capabilities (add a row here for new modules)
var ModuleCapabilities = []CapModule{
	{Key: "hosts", Actions: []string{"view", "create", "edit", "delete"}},
	{Key: "credentials", Actions: []string{"view", "manage"}},
	{Key: "reports", Actions: []string{"view"}},
	{Key: "monitor", Actions: []string{"view", "manage"}},
	{Key: "k8s", Actions: []string{"view", "manage"}},
	{Key: "crons", Actions: []string{"view", "manage"}},
	{Key: "apps", Actions: []string{"view", "create", "edit", "delete"}},
	{Key: "releases", Actions: []string{"view", "create", "rollback"}},
	{Key: "tasks", Actions: []string{"view", "export"}},
	{Key: "exec", Actions: []string{"exec"}},
	{Key: "files", Actions: []string{"distribute"}},
	{Key: "scripts", Actions: []string{"view", "manage", "exec"}},
	{Key: "keys", Actions: []string{"manage"}},
	{Key: "ai", Actions: []string{"chat"}},
}

// CapabilitiesForFront returns the role settings matrix for the frontend
func CapabilitiesForFront() []CapModule {
	out := make([]CapModule, len(ModuleCapabilities))
	copy(out, ModuleCapabilities)
	return out
}

// HasCap unified permission check (admin always passes)
func HasCap(role, module, action string) bool {
	if role == model.RoleAdmin {
		return true
	}
	r, ok := GetRoleSettings()[role]
	if !ok {
		return false
	}
	if len(r.Perms) == 0 {
		r.Perms = legacyToPerms(role, r)
	}
	for _, a := range r.Perms[module] {
		if a == action {
			return true
		}
	}
	return false
}

// legacyToPerms maps legacy scattered fields → Perms (one-time migration; new configs store Perms directly)
func legacyToPerms(role string, rp RolePerm) map[string][]string {
	inMenus := func(k string) bool {
		for _, m := range rp.Menus {
			if m == k {
				return true
			}
		}
		return false
	}
	p := map[string][]string{}

	h := []string{}
	if rp.Host.View {
		h = append(h, "view")
	}
	if rp.Host.Create {
		h = append(h, "create")
	}
	if rp.Host.Edit {
		h = append(h, "edit")
	}
	if rp.Host.Delete {
		h = append(h, "delete")
	}
	p["hosts"] = h

	if rp.Cred {
		p["credentials"] = []string{"view", "manage"}
	}
	if rp.Report {
		p["reports"] = []string{"view"}
	}
	k := []string{}
	if rp.K8sView || inMenus("k8s") {
		k = append(k, "view")
	}
	if rp.K8sManage {
		k = append(k, "manage")
	}
	p["k8s"] = k

	// monitor: read follows menus, manage=admin/ops (legacy route semantics)
	mv := []string{}
	if inMenus("monitor") {
		mv = append(mv, "view")
	}
	if role == model.RoleOps {
		mv = append(mv, "manage")
	}
	p["monitor"] = mv

	// apps / releases: read follows menus, write=admin/ops/publisher (legacy route semantics)
	aw := role == model.RoleOps || role == model.RolePublisher
	if inMenus("apps") {
		p["apps"] = []string{"view"}
		if aw {
			p["apps"] = []string{"view", "create", "edit", "delete"}
		}
	}
	if inMenus("releases") {
		p["releases"] = []string{"view"}
		if aw {
			p["releases"] = []string{"view", "create", "rollback"}
		}
	}

	// crons: read follows menus, manage=admin/ops (legacy route semantics)
	cv := []string{}
	if inMenus("cron") {
		cv = append(cv, "view")
	}
	if role == model.RoleOps {
		cv = append(cv, "manage")
	}
	p["crons"] = cv

	// tasks: read follows menus, export=admin/auditor (legacy route semantics)
	tv := []string{}
	if inMenus("tasks") {
		tv = append(tv, "view")
	}
	if role == model.RoleAdmin || role == model.RoleAuditor {
		tv = append(tv, "export")
	}
	p["tasks"] = tv

	// exec / files / scripts / keys: admin/ops (publisher for scripts too), read follows menus
	isOps := role == model.RoleOps
	isPub := role == model.RolePublisher
	if inMenus("exec") || isOps || isPub {
		p["exec"] = []string{"exec"}
	}
	if inMenus("files") || isOps || isPub {
		p["files"] = []string{"distribute"}
	}
	sv := []string{}
	if inMenus("scripts") {
		sv = append(sv, "view")
	}
	if isOps {
		sv = append(sv, "manage", "exec")
	} else if isPub {
		sv = append(sv, "exec")
	}
	p["scripts"] = sv
	if isOps {
		p["keys"] = []string{"manage"}
	}

	return p
}
