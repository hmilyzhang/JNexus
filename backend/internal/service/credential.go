// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"fmt"

	"autoops/internal/model"
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
	// 仅用户组直接关联的凭据
	var ids []uint
	model.DB.Table("user_group_credentials ugc").
		Joins("JOIN user_group_members ugm ON ugm.user_group_id = ugc.user_group_id").
		Where("ugm.user_id = ?", user.ID).
		Pluck("ugc.credential_id", &ids)
	if len(ids) == 0 {
		return nil
	}
	var usable []model.HostCredential
	for _, c := range creds {
		for _, id := range ids {
			if c.ID == id {
				usable = append(usable, c)
				break
			}
		}
	}
	return usable
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
