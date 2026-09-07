// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"jnexus/internal/model"
)

// 应用级数据权限：基于用户组 ↔ 应用绑定。
// admin/ops 平台级角色不受限；其他角色（publisher/viewer/自定义）按所属用户组的应用绑定过滤。

// UserGroupIDsOf 用户所属的用户组 ID 列表
func UserGroupIDsOf(userID uint) []uint {
	var ids []uint
	model.DB.Model(&model.UserGroupMember{}).Where("user_id = ?", userID).Pluck("user_group_id", &ids)
	return ids
}

// PlatformRole 平台级角色（不受数据级过滤）：admin 恒真，ops 单独判断
func PlatformRole(role string) bool {
	return role == model.RoleAdmin || role == model.RoleOps
}

// VisibleAppIDs 用户可见的应用 ID 集合；unrestricted=true 表示不做过滤（admin/ops）
func VisibleAppIDs(user *model.User) (ids map[uint]bool, unrestricted bool) {
	if PlatformRole(user.Role) {
		return nil, true
	}
	ids = map[uint]bool{}
	gids := UserGroupIDsOf(user.ID)
	if len(gids) == 0 {
		return ids, false
	}
	// Pluck 目标必须是切片（map 会触发 Scan 错误）
	var appIDs []uint
	model.DB.Model(&model.UserGroupApp{}).Where("user_group_id IN ?", gids).Pluck("app_id", &appIDs)
	for _, id := range appIDs {
		ids[id] = true
	}
	return ids, false
}

// DebugAccess 月报排障用：输出用户组成员与应用绑定（临时）
func DebugAccess(userID uint) (gids []uint, apps []uint) {
	gids = UserGroupIDsOf(userID)
	model.DB.Model(&model.UserGroupApp{}).Where("user_group_id IN ?", gids).Pluck("app_id", &apps)
	return
}

// CanAccessApp 用户能否访问（查看）指定应用
func CanAccessApp(user *model.User, appID uint) bool {
	ids, unrestricted := VisibleAppIDs(user)
	return unrestricted || ids[appID]
}

// CanOperateApp 用户能否对应用执行写操作（更新配置/创建发布/回滚）：
// 需 apps/releases 对应能力位（路由层已查）+ 所属用户组绑定了该应用
func CanOperateApp(user *model.User, appID uint) bool {
	if PlatformRole(user.Role) {
		return true
	}
	gids := UserGroupIDsOf(user.ID)
	if len(gids) == 0 {
		return false
	}
	var cnt int64
	model.DB.Model(&model.UserGroupApp{}).
		Where("user_group_id IN ? AND app_id = ?", gids, appID).Count(&cnt)
	return cnt > 0
}

// HostVisibilityFilter 主机列表可见性：返回过滤后的主机切片。
// admin/ops 或无 restrict_visibility 组的成员不受限；开启开关的组成员仅见本组绑定的主机/主机分组内主机。
func HostVisibilityFilter(user *model.User, hosts []model.Host) []model.Host {
	if PlatformRole(user.Role) {
		return hosts
	}
	gids := UserGroupIDsOf(user.ID)
	if len(gids) == 0 {
		return hosts
	}
	var restricted int64
	model.DB.Model(&model.UserGroup{}).Where("id IN ? AND restrict_visibility = ?", gids, true).Count(&restricted)
	if restricted == 0 {
		return hosts
	}
	// 允许的主机集合 = 直接绑定主机 + 绑定主机分组（含子分组）内的主机
	allowed := map[uint]bool{}
	var direct []uint
	model.DB.Model(&model.UserGroupHost{}).Where("user_group_id IN ?", gids).Pluck("host_id", &direct)
	for _, id := range direct {
		allowed[id] = true
	}
	var hgIDs []uint
	model.DB.Model(&model.UserGroupHostGroup{}).Where("user_group_id IN ?", gids).Pluck("host_group_id", &hgIDs)
	if len(hgIDs) > 0 {
		// 组下全部主机（含子分组级联）
		var hostIDs []uint
		model.DB.Table("hosts").
			Joins("JOIN host_groups hg ON hosts.group_id = hg.id").
			Where("hosts.group_id IN ? "+
				"OR hosts.group_id IN (SELECT id FROM host_groups WHERE parent_id IN ?)"+
				" OR hosts.group_id IN (SELECT id FROM host_groups WHERE parent_id IN (SELECT id FROM host_groups WHERE parent_id IN ?))",
				hgIDs, hgIDs, hgIDs).Pluck("hosts.id", &hostIDs)
		for _, id := range hostIDs {
			allowed[id] = true
		}
	}
	out := make([]model.Host, 0, len(allowed))
	for _, h := range hosts {
		if allowed[h.ID] {
			out = append(out, h)
		}
	}
	return out
}
