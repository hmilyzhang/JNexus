// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"jnexus/internal/model"
)

// 能力位统一权限模型：
// 每个模块在 ModuleCapabilities 注册自己支持的操作，路由用 RequireCap(module, action) 校验，
// 角色配置用 Perms[module] = [action...] 表达授权。菜单 key 与模块 key 同源，保证菜单与 API 一致。

// CapModule 前端角色设置矩阵所需的模块/操作声明
type CapModule struct {
	Key     string   `json:"key"`
	Actions []string `json:"actions"`
}

// ModuleCapabilities 全部模块能力注册表（新增模块在此加一行）
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
}

// CapabilitiesForFront 返回给前端角色设置矩阵
func CapabilitiesForFront() []CapModule {
	out := make([]CapModule, len(ModuleCapabilities))
	copy(out, ModuleCapabilities)
	return out
}

// HasCap 统一权限判断（admin 恒通过）
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

// legacyToPerms 旧散落字段 → Perms 映射（一次性迁移；新配置直接存 Perms）
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

	// monitor：读跟菜单，管理=admin/ops（旧路由语义）
	mv := []string{}
	if inMenus("monitor") {
		mv = append(mv, "view")
	}
	if role == model.RoleOps {
		mv = append(mv, "manage")
	}
	p["monitor"] = mv

	// apps / releases：读跟菜单，写=admin/ops/publisher（旧路由语义）
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

	// crons：读跟菜单，管理=admin/ops（旧路由语义）
	cv := []string{}
	if inMenus("cron") {
		cv = append(cv, "view")
	}
	if role == model.RoleOps {
		cv = append(cv, "manage")
	}
	p["crons"] = cv

	// tasks：读跟菜单，导出=admin/auditor（旧路由语义）
	tv := []string{}
	if inMenus("tasks") {
		tv = append(tv, "view")
	}
	if role == model.RoleAdmin || role == model.RoleAuditor {
		tv = append(tv, "export")
	}
	p["tasks"] = tv

	// exec / files / scripts / keys：admin/ops（scripts/publisher 也用），读跟菜单
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
