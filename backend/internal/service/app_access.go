// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"jnexus/internal/model"
)

// App-level data permissions: based on user group ↔ app bindings.
// Platform-level roles (admin/ops) are unrestricted; other roles (publisher/viewer/custom) are filtered by the app bindings of their user groups.

// UserGroupIDsOf returns the IDs of the user groups the user belongs to
func UserGroupIDsOf(userID uint) []uint {
	var ids []uint
	model.DB.Model(&model.UserGroupMember{}).Where("user_id = ?", userID).Pluck("user_group_id", &ids)
	return ids
}

// PlatformRole platform-level roles (exempt from data-level filtering): admin always true, ops checked separately
func PlatformRole(role string) bool {
	return role == model.RoleAdmin || role == model.RoleOps
}

// VisibleAppIDs set of app IDs visible to the user; unrestricted=true means no filtering (admin/ops)
func VisibleAppIDs(user *model.User) (ids map[uint]bool, unrestricted bool) {
	if PlatformRole(user.Role) {
		return nil, true
	}
	ids = map[uint]bool{}
	gids := UserGroupIDsOf(user.ID)
	if len(gids) == 0 {
		return ids, false
	}
	// Pluck target must be a slice (a map triggers a Scan error)
	var appIDs []uint
	model.DB.Model(&model.UserGroupApp{}).Where("user_group_id IN ?", gids).Pluck("app_id", &appIDs)
	for _, id := range appIDs {
		ids[id] = true
	}
	return ids, false
}

// DebugAccess for monthly-report troubleshooting: dumps user group membership and app bindings (temporary)
func DebugAccess(userID uint) (gids []uint, apps []uint) {
	gids = UserGroupIDsOf(userID)
	model.DB.Model(&model.UserGroupApp{}).Where("user_group_id IN ?", gids).Pluck("app_id", &apps)
	return
}

// CanAccessApp whether the user can access (view) the given app
func CanAccessApp(user *model.User, appID uint) bool {
	ids, unrestricted := VisibleAppIDs(user)
	return unrestricted || ids[appID]
}

// CanOperateApp whether the user can perform write operations on the app (update config / create release / rollback):
// requires the corresponding apps/releases capability bits (checked at the router layer) + the user's group is bound to the app
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

// HostVisibilityFilter host list visibility: returns the filtered host slice.
// admin/ops and members of groups without restrict_visibility are unrestricted; members of groups with the switch on only see hosts bound to their groups / within their bound host groups.
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
	// Allowed hosts = directly bound hosts + hosts within bound host groups (including sub-groups)
	allowed := map[uint]bool{}
	var direct []uint
	model.DB.Model(&model.UserGroupHost{}).Where("user_group_id IN ?", gids).Pluck("host_id", &direct)
	for _, id := range direct {
		allowed[id] = true
	}
	var hgIDs []uint
	model.DB.Model(&model.UserGroupHostGroup{}).Where("user_group_id IN ?", gids).Pluck("host_group_id", &hgIDs)
	if len(hgIDs) > 0 {
		// All hosts under the groups (including sub-group cascade)
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
