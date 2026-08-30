// AutoOps 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"autoops/internal/model"
)

// ---- 用户管理 ----

func ListUsers(c *gin.Context) {
	var users []model.User
	model.DB.Find(&users)
	out := make([]gin.H, 0, len(users))
	for _, u := range users {
		var memberOf []uint
		model.DB.Model(&model.UserGroupMember{}).Where("user_id = ?", u.ID).Pluck("user_group_id", &memberOf)
		out = append(out, gin.H{
			"id": u.ID, "username": u.Username, "role": u.Role, "auth_source": u.AuthSource,
			"email": u.Email, "status": u.Status, "last_login_at": u.LastLoginAt,
			"created_at": u.CreatedAt, "created_by": u.CreatedBy,
			"updated_by": u.UpdatedBy, "updated_at": u.UpdatedAt,
			"disabled_at": u.DisabledAt, "disabled_by": u.DisabledBy,
			"member_of": memberOf,
		})
	}
	c.JSON(http.StatusOK, out)
}

func CreateUser(c *gin.Context) {
	var req struct {
		Username     string `json:"username" binding:"required,min=2"`
		Password     string `json:"password" binding:"required,min=6"`
		Role         string `json:"role" binding:"required"`
		UserGroupIDs []uint `json:"user_group_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（用户名≥2位，密码≥6位）"})
		return
	}
	validRoles := map[string]bool{model.RoleAdmin: true, model.RoleOps: true, model.RolePublisher: true, model.RoleViewer: true, model.RoleAuditor: true}
	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法角色"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	u := model.User{Username: req.Username, Password: string(hash), Role: req.Role, Status: 1,
		CreatedBy: currentUser(c).Username, UpdatedBy: currentUser(c).Username}
	if err := model.DB.Create(&u).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名已存在"})
		return
	}
	for _, gid := range req.UserGroupIDs {
		model.DB.Create(&model.UserGroupMember{UserGroupID: gid, UserID: u.ID})
	}
	c.JSON(http.StatusOK, u)
}

func UpdateUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var u model.User
	if err := model.DB.First(&u, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	var req struct {
		Role         *string `json:"role"`
		Status       *int    `json:"status"`
		UserGroupIDs *[]uint `json:"user_group_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	op := currentUser(c).Username
	updates := map[string]any{"updated_by": op, "updated_at": time.Now()}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.Status != nil {
		updates["status"] = *req.Status
		if *req.Status == 0 { // 禁用：记录操作人与时间
			now := time.Now()
			updates["disabled_at"] = now
			updates["disabled_by"] = op
		} else { // 启用：清空
			updates["disabled_at"] = nil
			updates["disabled_by"] = ""
		}
	}
	model.DB.Model(&u).Updates(updates)
	if req.UserGroupIDs != nil {
		model.DB.Where("user_id = ?", u.ID).Delete(&model.UserGroupMember{})
		for _, gid := range *req.UserGroupIDs {
			model.DB.Create(&model.UserGroupMember{UserGroupID: gid, UserID: u.ID})
		}
	}
	c.JSON(http.StatusOK, u)
}

func DeleteUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	u := currentUser(c)
	if uint(id) == u.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除自己"})
		return
	}
	model.DB.Delete(&model.User{}, id)
	model.DB.Where("user_id = ?", id).Delete(&model.UserHostGroup{})
	model.DB.Where("user_id = ?", id).Delete(&model.UserApp{})
	model.DB.Where("user_id = ?", id).Delete(&model.UserGroupMember{})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---- 数据级授权 ----

type grantsReq struct {
	HostGroups []struct {
		GroupID   uint `json:"group_id"`
		CanExec   bool `json:"can_exec"`
		CanDeploy bool `json:"can_deploy"`
	} `json:"host_groups"`
	Apps []uint `json:"apps"`
}

func SetUserGrants(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req grantsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error { return doSetGrants(tx, uint(id), &req) })
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	refreshUserAudit(c, uint(id))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func doSetGrants(tx *gorm.DB, userID uint, req *grantsReq) error {
	if err := tx.Where("user_id = ?", userID).Delete(&model.UserHostGroup{}).Error; err != nil {
		return err
	}
	if err := tx.Where("user_id = ?", userID).Delete(&model.UserApp{}).Error; err != nil {
		return err
	}
	for _, g := range req.HostGroups {
		if err := tx.Create(&model.UserHostGroup{
			UserID: userID, GroupID: g.GroupID, CanExec: g.CanExec, CanDeploy: g.CanDeploy,
		}).Error; err != nil {
			return err
		}
	}
	for _, appID := range req.Apps {
		if err := tx.Create(&model.UserApp{UserID: userID, AppID: appID}).Error; err != nil {
			return err
		}
	}
	return nil
}

// refreshUserAudit 刷新用户「最近修改」审计字段
func refreshUserAudit(c *gin.Context, userID uint) {
	model.DB.Model(&model.User{}).Where("id = ?", userID).
		Updates(map[string]any{"updated_by": currentUser(c).Username, "updated_at": time.Now()})
}

func GetUserGrants(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var hgs []model.UserHostGroup
	model.DB.Where("user_id = ?", id).Find(&hgs)
	var apps []model.UserApp
	model.DB.Where("user_id = ?", id).Find(&apps)
	c.JSON(http.StatusOK, gin.H{"host_groups": hgs, "apps": apps})
}

// GrantsOptions 提供授权下拉数据（分组列表 + 应用列表）
func GrantsOptions(c *gin.Context) {
	var groups []model.HostGroup
	model.DB.Find(&groups)
	var apps []model.Application
	model.DB.Select("id, name, description").Find(&apps)
	c.JSON(http.StatusOK, gin.H{"host_groups": groups, "apps": apps})
}
