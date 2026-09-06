// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"fmt"

	"jnexus/internal/model"
)

// UsableCredentials 用户在某主机上可用的 OS 账号列表：
// 拥有主机级访问权限（个人授权/用户组关联主机）→ 该主机全部凭据；
// 否则仅限通过用户组直接关联的凭据
func UsableCredentials(user *model.User, host *model.Host) []model.HostCredential {
	var creds []model.HostCredential
	model.DB.Where("host_id = ?", host.ID).Order("is_default DESC, id ASC").Find(&creds)
	if len(creds) == 0 {
		return nil
	}
	if user.IsAdmin() || CanExecHost(user, host.ID, host.GroupID) {
		return creds
	}
	// 用户组授权：直接关联的凭据 + 账号规则（主机分组范围 × 账号名）
	var linkedIDs []uint
	model.DB.Table("user_group_credentials ugc").
		Joins("JOIN user_group_members ugm ON ugm.user_group_id = ugc.user_group_id").
		Where("ugm.user_id = ?", user.ID).
		Pluck("ugc.credential_id", &linkedIDs)
	idSet := map[uint]bool{}
	for _, id := range linkedIDs {
		idSet[id] = true
	}

	// 用户所在组的账号规则
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

	// 主机所在分组的祖先链：规则命中任一祖先分组即生效（多级级联）
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

// UsableCredentialsAll 批量版 UsableCredentials：一次预取全部凭据与授权关系，
// 供 400+ 主机规模的列表接口使用（原逐主机版本每台 1-4 条查询，400 台 = 400-1600 条 SQL）。
// 返回 host_id -> 可用凭据列表。
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

	// 个人主机分组授权（祖先分组级联，can_exec）
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
	// 用户组直接关联主机
	var ugHostIDs []uint
	model.DB.Table("user_group_hosts ug_h").
		Joins("JOIN user_group_members ug_m ON ug_m.user_group_id = ug_h.user_group_id").
		Where("ug_m.user_id = ?", user.ID).Pluck("ug_h.host_id", &ugHostIDs)
	ugHostSet := map[uint]bool{}
	for _, id := range ugHostIDs {
		ugHostSet[id] = true
	}
	// 用户组关联主机分组
	var ugGroupIDs []uint
	model.DB.Table("user_group_host_groups ug_g").
		Joins("JOIN user_group_members ug_m ON ug_m.user_group_id = ug_g.user_group_id").
		Where("ug_m.user_id = ?", user.ID).Pluck("ug_g.host_group_id", &ugGroupIDs)
	ugGroupSet := map[uint]bool{}
	for _, id := range ugGroupIDs {
		ugGroupSet[id] = true
	}
	// 用户组直接关联凭据
	var linkedIDs []uint
	model.DB.Table("user_group_credentials ugc").
		Joins("JOIN user_group_members ugm ON ugm.user_group_id = ugc.user_group_id").
		Where("ugm.user_id = ?", user.ID).Pluck("ugc.credential_id", &linkedIDs)
	linkedSet := map[uint]bool{}
	for _, id := range linkedIDs {
		linkedSet[id] = true
	}
	// 用户组账号规则（主机分组范围 × 账号名）
	type credRule struct {
		HostGroupID *uint
		Username    string
	}
	var rules []credRule
	model.DB.Table("user_group_cred_rules ugr").
		Joins("JOIN user_group_members ugm ON ugm.user_group_id = ugr.user_group_id").
		Select("ugr.host_group_id, ugr.username").
		Where("ugm.user_id = ?", user.ID).Scan(&rules)
	// 全部分组：内存计算祖先链（避免逐主机查库）
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

// ResolveCredential 为用户解析主机上要使用的 OS 账号：
// 显式指定 credential_id 时必须可用；否则取默认可用账号
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
		// 显式指定的凭据不在可用列表：可能存在但无权
		var cnt int64
		model.DB.Model(&model.HostCredential{}).Where("id = ? AND host_id = ?", *explicitID, host.ID).Count(&cnt)
		if cnt > 0 {
			return nil, fmt.Errorf("无权使用该 OS 账号")
		}
		return nil, fmt.Errorf("OS 账号不存在或不属于该主机")
	}
	return &usable[0], nil // 默认账号优先（is_default DESC, id ASC）
}
