package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"autoops/internal/model"
)

// ---- 用户管理 ----

func ListUsers(c *gin.Context) {
	var users []model.User
	model.DB.Find(&users)
	c.JSON(http.StatusOK, users)
}

func CreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=2"`
		Password string `json:"password" binding:"required,min=6"`
		Role     string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（用户名≥2位，密码≥6位）"})
		return
	}
	validRoles := map[string]bool{model.RoleAdmin: true, model.RoleOps: true, model.RolePublisher: true, model.RoleViewer: true}
	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法角色"})
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	u := model.User{Username: req.Username, Password: string(hash), Role: req.Role, Status: 1}
	if err := model.DB.Create(&u).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名已存在"})
		return
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
		Role   *string `json:"role"`
		Status *int    `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	updates := map[string]any{}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	model.DB.Model(&u).Updates(updates)
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
