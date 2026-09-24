// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

// Database workbench endpoints: run day-to-day SQL against registered database
// sources as one of their accounts. Sources and accounts are admin-managed;
// account visibility for regular users is limited by AllowedGroups (CSV of
// user-group IDs, empty = all). Every query execution is audited.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
)

// userGroupIDsOf returns the user-group IDs the user belongs to
func userGroupIDsOf(userID uint) []uint {
	var ids []uint
	model.DB.Table("user_group_members").Where("user_id = ?", userID).Pluck("user_group_id", &ids)
	return ids
}

// accountAllowedFor checks the account's AllowedGroups against the user's groups
// (admins bypass the restriction)
func accountAllowedFor(user *model.User, account model.DBAccount) bool {
	if user.IsAdmin() || strings.TrimSpace(account.AllowedGroups) == "" {
		return true
	}
	userGroups := userGroupIDsOf(user.ID)
	if len(userGroups) == 0 {
		return false
	}
	allowed := map[uint]bool{}
	for _, part := range strings.Split(account.AllowedGroups, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil {
			allowed[uint(n)] = true
		}
	}
	for _, g := range userGroups {
		if allowed[g] {
			return true
		}
	}
	return false
}

func dbAccountOut(a model.DBAccount) gin.H {
	return gin.H{"id": a.ID, "source_id": a.SourceID, "username": a.Username,
		"label": a.Label, "allowed_groups": a.AllowedGroups, "created_at": a.CreatedAt}
}

// DBSourceListForWorkbench GET /api/databases/sources - registered sources for
// the workbench (connection passwords masked)
func DBSourceListForWorkbench(c *gin.Context) {
	out := make([]gin.H, 0)
	for _, s := range service.ListDbSources() {
		out = append(out, gin.H{
			"id": s.ID, "name": s.Name, "db_type": s.DBType, "host": s.Host,
			"port": s.Port, "database": s.Database, "read_only": s.ReadOnly,
			"timeout_sec": s.TimeoutSec, "max_rows": s.MaxRows,
		})
	}
	c.JSON(http.StatusOK, out)
}

// CreateDBSource POST /api/databases/sources - admin only; registers a source
// for the workbench (enabled=false, no ingestion query)
func CreateDBSource(c *gin.Context) {
	var req struct {
		Name       string `json:"name"`
		DBType     string `json:"db_type"`
		Host       string `json:"host"`
		Port       int    `json:"port"`
		Database   string `json:"database"`
		ReadOnly   bool   `json:"read_only"`
		TimeoutSec int    `json:"timeout_sec"`
		MaxRows    int    `json:"max_rows"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	id, err := service.CreateWorkbenchSource(req.Name, req.DBType, req.Host, req.Port, req.Database, req.ReadOnly, req.TimeoutSec, req.MaxRows)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// DBSourceGuardrails PUT /api/databases/sources/:id/guardrails - admin only
func DBSourceGuardrails(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		ReadOnly   bool `json:"read_only"`
		TimeoutSec int  `json:"timeout_sec"`
		MaxRows    int  `json:"max_rows"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		return
	}
	updates := map[string]any{"read_only": req.ReadOnly, "timeout_sec": req.TimeoutSec, "max_rows": req.MaxRows}
	if err := model.DB.Model(&model.DbSource{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DBSourceAccounts GET /api/databases/:id/accounts — accounts of one source,
// filtered by the caller's permissions (admins see all)
func DBSourceAccounts(c *gin.Context) {
	u := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var accounts []model.DBAccount
	model.DB.Where("source_id = ?", id).Order("id").Find(&accounts)
	out := make([]gin.H, 0, len(accounts))
	for _, a := range accounts {
		if accountAllowedFor(u, a) {
			out = append(out, dbAccountOut(a))
		}
	}
	c.JSON(http.StatusOK, out)
}

// CreateDBAccount POST /api/databases/:id/accounts — admin only
// RotateDBAccount POST /databases/sources/:sid/accounts/:aid/rotate - admin:
// rotate one database account now via the source's designated rotator account.
func RotateDBAccount(c *gin.Context) {
	aid, _ := strconv.Atoi(c.Param("aid"))
	if err := service.RotateDBAccountNow(uint(aid), currentUser(c).Username); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// UpdateDBAccountRotation PUT /databases/accounts/:aid/rotation-settings - admin:
// per-account rotation toggle/period and the source's designated rotator flag.
func UpdateDBAccountRotation(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var a model.DBAccount
	if err := model.DB.First(&a, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}
	var req struct {
		RotateEnabled *bool `json:"rotate_enabled"`
		RotateDays    *int  `json:"rotate_days"`
		IsRotator     *bool `json:"is_rotator"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	updates := map[string]any{}
	if req.RotateEnabled != nil {
		updates["rotate_enabled"] = *req.RotateEnabled
	}
	if req.RotateDays != nil {
		if *req.RotateDays < 0 || *req.RotateDays > 3650 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "周期须在 0-3650 天"})
			return
		}
		updates["rotate_days"] = *req.RotateDays
	}
	if req.IsRotator != nil {
		if *req.IsRotator {
			// only one rotator per source; the rotator itself is excluded from rotation
			model.DB.Model(&model.DBAccount{}).
				Where("source_id = ? AND is_rotator = ?", a.SourceID, true).
				Update("is_rotator", false)
			updates["is_rotator"] = true
			updates["rotate_enabled"] = false
		} else {
			updates["is_rotator"] = false
		}
	}
	model.DB.Model(&a).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func CreateDBAccount(c *gin.Context) {
	sourceID, _ := strconv.Atoi(c.Param("id"))
	var src model.DbSource
	if err := model.DB.First(&src, sourceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据库源不存在"})
		return
	}
	var req struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		Label         string `json:"label"`
		AllowedGroups string `json:"allowed_groups"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码必填"})
		return
	}
	enc, err := pkg.Encrypt(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a := model.DBAccount{
		SourceID: uint(sourceID), Username: req.Username, Password: enc,
		Label: req.Label, AllowedGroups: req.AllowedGroups, CreatedAt: time.Now(),
	}
	if err := model.DB.Create(&a).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, dbAccountOut(a))
}

// UpdateDBAccount PUT /api/databases/accounts/:id — admin only; empty password keeps it
func UpdateDBAccount(c *gin.Context) {
	aid, _ := strconv.Atoi(c.Param("id"))
	var a model.DBAccount
	if err := model.DB.First(&a, aid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}
	var req struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		Label         string `json:"label"`
		AllowedGroups string `json:"allowed_groups"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名必填"})
		return
	}
	updates := map[string]any{"username": req.Username, "label": req.Label, "allowed_groups": req.AllowedGroups}
	if req.Password != "" {
		enc, err := pkg.Encrypt(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		updates["password"] = enc
	}
	model.DB.Model(&a).Updates(updates)
	a.Username, a.Label, a.AllowedGroups = req.Username, req.Label, req.AllowedGroups
	c.JSON(http.StatusOK, dbAccountOut(a))
}

// DeleteDBAccount DELETE /api/databases/accounts/:id — admin only
func DeleteDBAccount(c *gin.Context) {
	aid, _ := strconv.Atoi(c.Param("id"))
	model.DB.Delete(&model.DBAccount{}, aid)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RunDBQuery POST /api/databases/:id/query {account_id, sql} — executes one
// statement as the chosen account (permission + guardrails applied, audited)
func RunDBQueryHandler(c *gin.Context) {
	u := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		AccountID uint   `json:"account_id"`
		SQL       string `json:"sql"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.SQL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "SQL 必填"})
		return
	}
	var src model.DbSource
	if err := model.DB.First(&src, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据库源不存在"})
		return
	}
	var account model.DBAccount
	if err := model.DB.First(&account, req.AccountID).Error; err != nil || account.SourceID != src.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "账号不存在或不属于该数据库源"})
		return
	}
	if !accountAllowedFor(u, account) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权使用该数据库账号"})
		return
	}
	password, derr := pkg.Decrypt(account.Password)
	if derr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解密失败: " + derr.Error()})
		return
	}
	res, qerr := service.RunDBQuery(&src, account.Username, password, req.SQL)
	status := http.StatusOK
	if qerr != nil {
		status = http.StatusBadRequest
	}
	model.DB.Create(&model.AuditLog{
		UserID: u.ID, Username: u.Username,
		Action: "DB_QUERY", Resource: "/api/databases/" + strconv.Itoa(id),
		Detail: `{"account":"` + account.Username + `","sql":` + jsonStringOf(req.SQL) + `,"ok":` + map[bool]string{true: "true", false: "false"}[qerr == nil] + `}`,
		IP:     c.ClientIP(), Status: map[bool]int{true: 200, false: 400}[qerr == nil], CreatedAt: time.Now(),
	})
	if qerr != nil {
		c.JSON(status, gin.H{"error": qerr.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// jsonStringOf encodes a Go string as a JSON string literal (safe embedding)
func jsonStringOf(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// dbSchemaCacheEntry caches table/column metadata per source+account
type dbSchemaCacheEntry struct {
	tables    []gin.H
	fetchedAt time.Time
}

var dbSchemaCacheMu sync.Mutex
var dbSchemaCache = map[string]dbSchemaCacheEntry{}

// DBSchema GET /api/databases/:id/schema?account_id= — table/column metadata
// used by the SQL editor completion (cached 5 minutes per source+account)
func DBSchema(c *gin.Context) {
	u := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	accountID, _ := strconv.Atoi(c.Query("account_id"))
	var src model.DbSource
	if err := model.DB.First(&src, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据库源不存在"})
		return
	}
	var account model.DBAccount
	if err := model.DB.First(&account, accountID).Error; err != nil || account.SourceID != src.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "账号不存在或不属于该数据库源"})
		return
	}
	if !accountAllowedFor(u, account) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权使用该数据库账号"})
		return
	}
	cacheKey := fmt.Sprintf("%d:%d", id, accountID)
	dbSchemaCacheMu.Lock()
	entry, cached := dbSchemaCache[cacheKey]
	dbSchemaCacheMu.Unlock()
	if cached && time.Since(entry.fetchedAt) < 5*time.Minute {
		c.JSON(http.StatusOK, gin.H{"tables": entry.tables, "cached": true})
		return
	}
	password, derr := pkg.Decrypt(account.Password)
	if derr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解密失败: " + derr.Error()})
		return
	}
	db, err := service.OpenDB(&src, account.Username, password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "连接失败: " + err.Error()})
		return
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var query string
	switch src.DBType {
	case "mysql":
		query = "SELECT table_name, column_name FROM information_schema.columns WHERE table_schema = DATABASE() ORDER BY table_name, ordinal_position"
	case "pgsql":
		query = "SELECT table_name, column_name FROM information_schema.columns WHERE table_schema NOT IN ('pg_catalog','information_schema') ORDER BY table_name, ordinal_position"
	case "oracle":
		query = "SELECT table_name, column_name FROM user_tab_columns ORDER BY table_name, column_id"
	default:
		query = "SELECT TABLE_NAME, COLUMN_NAME FROM INFORMATION_SCHEMA.COLUMNS ORDER BY TABLE_NAME, ORDINAL_POSITION"
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "元数据查询失败: " + err.Error()})
		return
	}
	defer rows.Close()
	type tbl struct {
		name    string
		columns []string
	}
	var order []string
	byName := map[string]*tbl{}
	for rows.Next() {
		var tn, cn string
		if err := rows.Scan(&tn, &cn); err != nil {
			continue
		}
		t, ok := byName[tn]
		if !ok {
			t = &tbl{name: tn}
			byName[tn] = t
			order = append(order, tn)
		}
		t.columns = append(t.columns, cn)
	}
	tables := make([]gin.H, 0, len(order))
	for _, tn := range order {
		tables = append(tables, gin.H{"name": tn, "columns": byName[tn].columns})
	}
	dbSchemaCacheMu.Lock()
	dbSchemaCache[cacheKey] = dbSchemaCacheEntry{tables: tables, fetchedAt: time.Now()}
	dbSchemaCacheMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"tables": tables})
}
