// JNexus 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"autoops/internal/model"
)

// ---- 用户组管理 ----

type userGroupSummary struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Members     int64  `json:"members"`
	Hosts       int64  `json:"hosts"`
	HostGroups  int64  `json:"host_groups"`
	CreatedAt   string `json:"created_at"`
}

func ListUserGroups(c *gin.Context) {
	var groups []model.UserGroup
	model.DB.Order("id").Find(&groups)
	out := make([]userGroupSummary, 0, len(groups))
	for _, g := range groups {
		s := userGroupSummary{ID: g.ID, Name: g.Name, Description: g.Description, CreatedAt: g.CreatedAt.Format("2006-01-02 15:04:05")}
		model.DB.Model(&model.UserGroupMember{}).Where("user_group_id = ?", g.ID).Count(&s.Members)
		model.DB.Model(&model.UserGroupHost{}).Where("user_group_id = ?", g.ID).Count(&s.Hosts)
		model.DB.Model(&model.UserGroupHostGroup{}).Where("user_group_id = ?", g.ID).Count(&s.HostGroups)
		out = append(out, s)
	}
	c.JSON(http.StatusOK, out)
}

func CreateUserGroup(c *gin.Context) {
	var g model.UserGroup
	if err := c.ShouldBindJSON(&g); err != nil || g.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户组名不能为空"})
		return
	}
	if err := model.DB.Create(&g).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户组名已存在"})
		return
	}
	c.JSON(http.StatusOK, g)
}

func UpdateUserGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var g model.UserGroup
	if err := model.DB.First(&g, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户组不存在"})
		return
	}
	var req model.UserGroup
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户组名不能为空"})
		return
	}
	model.DB.Model(&g).Updates(map[string]any{"name": req.Name, "description": req.Description})
	c.JSON(http.StatusOK, g)
}

func DeleteUserGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	model.DB.Where("user_group_id = ?", id).Delete(&model.UserGroupMember{})
	model.DB.Where("user_group_id = ?", id).Delete(&model.UserGroupHost{})
	model.DB.Where("user_group_id = ?", id).Delete(&model.UserGroupHostGroup{})
	model.DB.Where("user_group_id = ?", id).Delete(&model.UserGroupCredRule{})
	model.DB.Delete(&model.UserGroup{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetUserGroup 用户组详情（含成员/主机/主机分组 ID 列表）
func GetUserGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var g model.UserGroup
	if err := model.DB.First(&g, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户组不存在"})
		return
	}
	var memberIDs, hostIDs, groupIDs, credIDs []uint
	model.DB.Model(&model.UserGroupMember{}).Where("user_group_id = ?", id).Pluck("user_id", &memberIDs)
	model.DB.Model(&model.UserGroupHost{}).Where("user_group_id = ?", id).Pluck("host_id", &hostIDs)
	model.DB.Model(&model.UserGroupHostGroup{}).Where("user_group_id = ?", id).Pluck("host_group_id", &groupIDs)
	model.DB.Model(&model.UserGroupCredential{}).Where("user_group_id = ?", id).Pluck("credential_id", &credIDs)
	type ruleRow struct {
		HostGroupID *uint  `json:"host_group_id"`
		Username    string `json:"username"`
	}
	var rules []ruleRow
	model.DB.Model(&model.UserGroupCredRule{}).Select("host_group_id", "username").
		Where("user_group_id = ?", id).Scan(&rules)
	c.JSON(http.StatusOK, gin.H{
		"id": g.ID, "name": g.Name, "description": g.Description,
		"member_ids": memberIDs, "host_ids": hostIDs, "host_group_ids": groupIDs,
		"credential_ids": credIDs, "rules": rules,
	})
}

func UpdateUserGroupLinks(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var g model.UserGroup
	if err := model.DB.First(&g, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户组不存在"})
		return
	}
	var req struct {
		MemberIDs     []uint `json:"member_ids"`
		HostIDs       []uint `json:"host_ids"`
		GroupIDs      []uint `json:"host_group_ids"`
		CredentialIDs []uint `json:"credential_ids"`
		Rules         []struct {
			HostGroupID *uint  `json:"host_group_id"`
			Username    string `json:"username" binding:"required"`
		} `json:"rules"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	oldMemberIDs, newMemberIDs := []uint{}, []uint{}
	model.DB.Model(&model.UserGroupMember{}).Where("user_group_id = ?", g.ID).Pluck("user_id", &oldMemberIDs)
	model.DB.Where("user_group_id = ?", g.ID).Delete(&model.UserGroupMember{})
	for _, v := range req.MemberIDs {
		model.DB.Create(&model.UserGroupMember{UserGroupID: g.ID, UserID: v})
		newMemberIDs = append(newMemberIDs, v)
	}
	// 成员变化刷新双方用户的「最近修改」审计字段
	op := currentUser(c).Username
	now := time.Now()
	touched := map[uint]bool{}
	for _, id := range append(oldMemberIDs, newMemberIDs...) {
		if !touched[id] {
			model.DB.Model(&model.User{}).Where("id = ?", id).
				Updates(map[string]any{"updated_by": op, "updated_at": now})
			touched[id] = true
		}
	}
	model.DB.Where("user_group_id = ?", g.ID).Delete(&model.UserGroupHost{})
	for _, v := range req.HostIDs {
		model.DB.Create(&model.UserGroupHost{UserGroupID: g.ID, HostID: v})
	}
	model.DB.Where("user_group_id = ?", g.ID).Delete(&model.UserGroupHostGroup{})
	for _, v := range req.GroupIDs {
		model.DB.Create(&model.UserGroupHostGroup{UserGroupID: g.ID, HostGroupID: v})
	}
	model.DB.Where("user_group_id = ?", g.ID).Delete(&model.UserGroupCredential{})
	for _, v := range req.CredentialIDs {
		model.DB.Create(&model.UserGroupCredential{UserGroupID: g.ID, CredentialID: v})
	}
	model.DB.Where("user_group_id = ?", g.ID).Delete(&model.UserGroupCredRule{})
	for _, r := range req.Rules {
		if strings.TrimSpace(r.Username) == "" {
			continue
		}
		model.DB.Create(&model.UserGroupCredRule{UserGroupID: g.ID, HostGroupID: r.HostGroupID, Username: strings.TrimSpace(r.Username)})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
