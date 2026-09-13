// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"fmt"

	"jnexus/internal/model"
)

// UsableCredentials list of OS accounts usable by the user on a given host:
// with host-level access (personal grant / user-group linked hosts) → all credentials of that host;
// otherwise only credentials directly linked via user groups
func UsableCredentials(user *model.User, host *model.Host) []model.HostCredential {
	var creds []model.HostCredential
	model.DB.Where("host_id = ?", host.ID).Order("is_default DESC, id ASC").Find(&creds)
	if len(creds) == 0 {
		return nil
	}
	if user.IsAdmin() || CanExecHost(user, host.ID, host.GroupID) {
		return creds
	}
	// User group grants: directly linked credentials + account rules (host group scope × account name)
	var linkedIDs []uint
	model.DB.Table("user_group_credentials ugc").
		Joins("JOIN user_group_members ugm ON ugm.user_group_id = ugc.user_group_id").
		Where("ugm.user_id = ?", user.ID).
		Pluck("ugc.credential_id", &linkedIDs)
	idSet := map[uint]bool{}
	for _, id := range linkedIDs {
		idSet[id] = true
	}

	// Account rules of the user's groups
	type credRule struct {
		HostGroupID *uint
		Username    string
	}
	var rules []credRule
	model.DB.Table("user_group_cred_rules ugr").
		Joins("JOIN user_group_members ugm ON ugm.user_group_id = ugr.user_group_id").
		Select("ugr.host_group_id, ugr.username").
		Where("ugm.user_id = ?", user.ID).
		Scan(&rules)

	// Ancestor chain of the host's group: a rule matching any ancestor group applies (multi-level cascade)
	var ancestorSet map[uint]bool
	if host.GroupID != nil {
		ancestorSet = map[uint]bool{}
		for _, id := range GroupAncestors(*host.GroupID) {
			ancestorSet[id] = true
		}
	}

	var usable []model.HostCredential
	for _, c := range creds {
		ok := idSet[c.ID]
		if !ok {
			for _, r := range rules {
				inScope := r.HostGroupID == nil || (ancestorSet != nil && ancestorSet[*r.HostGroupID])
				if inScope && r.Username == c.Username {
					ok = true
					break
				}
			}
		}
		if ok {
			usable = append(usable, c)
		}
	}
	return usable
}

// UsableCredentialsAll batch version of UsableCredentials: prefetches all credentials and grant relations at once,
// for list endpoints at 400+ host scale (the per-host version ran 1-4 queries per host; 400 hosts = 400-1600 SQL queries).
// Returns host_id -> list of usable credentials.
func UsableCredentialsAll(user *model.User, hosts []model.Host) map[uint][]model.HostCredential {
	out := make(map[uint][]model.HostCredential, len(hosts))

	var creds []model.HostCredential
	model.DB.Order("host_id, is_default DESC, id ASC").Find(&creds)
	byHost := map[uint][]model.HostCredential{}
	for _, c := range creds {
		byHost[c.HostID] = append(byHost[c.HostID], c)
	}
	if user.IsAdmin() {
		for _, h := range hosts {
			out[h.ID] = byHost[h.ID]
		}
		return out
	}

	execOK := user.Role == model.RoleOps || user.Role == model.RolePublisher

	// Personal host group grants (ancestor group cascade, can_exec)
	var personal []struct {
		GroupID uint
		CanExec bool
	}
	model.DB.Table("user_host_groups").
		Select("group_id, can_exec").Where("user_id = ?", user.ID).Scan(&personal)
	personalSet := map[uint]bool{}
	for _, r := range personal {
		if r.CanExec {
			personalSet[r.GroupID] = true
		}
	}
	// Hosts directly linked by user groups
	var ugHostIDs []uint
	model.DB.Table("user_group_hosts ug_h").
		Joins("JOIN user_group_members ug_m ON ug_m.user_group_id = ug_h.user_group_id").
		Where("ug_m.user_id = ?", user.ID).Pluck("ug_h.host_id", &ugHostIDs)
	ugHostSet := map[uint]bool{}
	for _, id := range ugHostIDs {
		ugHostSet[id] = true
	}
	// Host groups linked by user groups
	var ugGroupIDs []uint
	model.DB.Table("user_group_host_groups ug_g").
		Joins("JOIN user_group_members ug_m ON ug_m.user_group_id = ug_g.user_group_id").
		Where("ug_m.user_id = ?", user.ID).Pluck("ug_g.host_group_id", &ugGroupIDs)
	ugGroupSet := map[uint]bool{}
	for _, id := range ugGroupIDs {
		ugGroupSet[id] = true
	}
	// Credentials directly linked by user groups
	var linkedIDs []uint
	model.DB.Table("user_group_credentials ugc").
		Joins("JOIN user_group_members ugm ON ugm.user_group_id = ugc.user_group_id").
		Where("ugm.user_id = ?", user.ID).Pluck("ugc.credential_id", &linkedIDs)
	linkedSet := map[uint]bool{}
	for _, id := range linkedIDs {
		linkedSet[id] = true
	}
	// User group account rules (host group scope × account name)
	type credRule struct {
		HostGroupID *uint
		Username    string
	}
	var rules []credRule
	model.DB.Table("user_group_cred_rules ugr").
		Joins("JOIN user_group_members ugm ON ugm.user_group_id = ugr.user_group_id").
		Select("ugr.host_group_id, ugr.username").
		Where("ugm.user_id = ?", user.ID).Scan(&rules)
	// All groups: compute ancestor chains in memory (avoids per-host DB queries)
	var groups []model.HostGroup
	model.DB.Find(&groups)
	parent := map[uint]*uint{}
	for _, g := range groups {
		parent[g.ID] = g.ParentID
	}
	ancestors := func(groupID uint) map[uint]bool {
		set := map[uint]bool{}
		cur := groupID
		for {
			set[cur] = true
			p := parent[cur]
			if p == nil {
				break
			}
			cur = *p
		}
		return set
	}

	for _, h := range hosts {
		credsOfHost := byHost[h.ID]
		if len(credsOfHost) == 0 {
			continue
		}
		var anc map[uint]bool
		if h.GroupID != nil {
			anc = ancestors(*h.GroupID)
		}
		canExec := false
		if execOK {
			if anc != nil {
				for gid := range anc {
					if personalSet[gid] {
						canExec = true
						break
					}
				}
			}
			if !canExec && ugHostSet[h.ID] {
				canExec = true
			}
			if !canExec && anc != nil {
				for gid := range anc {
					if ugGroupSet[gid] {
						canExec = true
						break
					}
				}
			}
		}
		if canExec {
			out[h.ID] = credsOfHost
			continue
		}
		var usable []model.HostCredential
		for _, c := range credsOfHost {
			ok := linkedSet[c.ID]
			if !ok {
				for _, r := range rules {
					inScope := r.HostGroupID == nil || (anc != nil && anc[*r.HostGroupID])
					if inScope && r.Username == c.Username {
						ok = true
						break
					}
				}
			}
			if ok {
				usable = append(usable, c)
			}
		}
		out[h.ID] = usable
	}
	return out
}

// ResolveCredential resolves the OS account to use on a host for the user:
// an explicit credential_id must be usable; otherwise the default usable account is picked
func ResolveCredential(user *model.User, host *model.Host, explicitID *uint) (*model.HostCredential, error) {
	usable := UsableCredentials(user, host)
	if len(usable) == 0 {
		return nil, fmt.Errorf("无可用的 OS 账号（请联系管理员分配）")
	}
	if explicitID != nil {
		for i := range usable {
			if usable[i].ID == *explicitID {
				return &usable[i], nil
			}
		}
		// Explicit credential is not in the usable list: it may exist but the user has no access
		var cnt int64
		model.DB.Model(&model.HostCredential{}).Where("id = ? AND host_id = ?", *explicitID, host.ID).Count(&cnt)
		if cnt > 0 {
			return nil, fmt.Errorf("无权使用该 OS 账号")
		}
		return nil, fmt.Errorf("OS 账号不存在或不属于该主机")
	}
	return &usable[0], nil // default account first (is_default DESC, id ASC)
}
