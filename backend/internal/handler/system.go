// JNexus Ops Platform — By JJ Zhang, Version 1.0

package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/buildinfo"
	"jnexus/internal/model"
	"jnexus/internal/service"
)

// containsStr reports whether the slice contains the exact string
func containsStr(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// System config keys
var editableConfigKeys = []string{
	"system_name",
	"ldap_enabled", "ldap_host", "ldap_port", "ldap_tls",
	"ldap_bind_dn", "ldap_bind_password", "ldap_base_dn",
	"ldap_user_filter", "ldap_attr_username", "ldap_default_role",
	"ldap_group_check", "ldap_group_base_dn", "ldap_group_filter", "ldap_required_groups",
	"smtp_enabled", "smtp_host", "smtp_port", "smtp_ssl", "smtp_tls",
	"smtp_username", "smtp_password", "smtp_from", "smtp_recipients", "smtp_notify",
	"rotation_enabled", "rotation_length", "rotation_complexity", "rotation_days",
	"ai_enabled", "ai_base_url", "ai_api_key", "ai_model", "ai_timeout_sec",
	"ai_system_prompt",
	"oo_enabled", "oo_url", "oo_org", "oo_token", "oo_integrations",
	"winrm_krb5_realm", "winrm_krb5_config",
	"ssh_key_rotation_enabled", "ssh_key_rotation_days", "ssh_key_rotation_last",
	"ai_chat_rate_limit", "ai_injection_guard", "ai_snapshot_filter",
	"sec_alert_channels", "sec_alert_ids",
	"oo_retention_days",
	"ticket_servicenow_url", "ticket_servicenow_user", "ticket_servicenow_pass",
	"ticket_sdp_url", "ticket_sdp_token", "ticket_sdp_requester",
}

// secret config keys: masked in responses, empty value on update keeps the stored one
var secretConfigKeys = []string{"ldap_bind_password", "smtp_password", "ai_api_key", "oo_token", "ticket_servicenow_pass", "ticket_sdp_token"}

// GetSystemConfig reads system config (admin); password fields are masked
func GetSystemConfig(c *gin.Context) {
	m := service.SystemConfigMap()
	for _, k := range secretConfigKeys {
		if m[k] != "" {
			m[k] = "******"
		}
	}
	c.JSON(http.StatusOK, m)
}

// UpdateSystemConfig updates system config (admin); empty/masked passwords keep the original value
func UpdateSystemConfig(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	old := service.SystemConfigMap()
	filtered := map[string]string{}
	for _, k := range editableConfigKeys {
		v, ok := req[k]
		if !ok {
			continue
		}
		isSecret := containsStr(secretConfigKeys, k)
		if isSecret && (v == "" || v == "******") {
			continue // keep original value
		}
		filtered[k] = v
	}
	if v, ok := filtered["ldap_enabled"]; ok && v != "true" && v != "false" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ldap_enabled 只能是 true/false"})
		return
	}
	if err := service.SetSystemConfigs(filtered); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	_ = old
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetSystemRoles returns role settings (readable by all logged-in users, used for menu/UI filtering)
func GetSystemRoles(c *gin.Context) {
	c.JSON(http.StatusOK, service.GetRoleSettings())
}

// UpdateSystemRoles saves role settings (admin)
// ---- Custom role management ----

// builtinRoles built-in role keys (cannot be deleted; admin additionally forbids all write operations)
var builtinRoles = map[string]bool{
	"admin": true, "ops": true, "publisher": true,
	"viewer": true, "auditor": true, "k8s": true,
}

// CreateRole POST /api/system/roles  creates a custom role (can copy permissions from an existing role)
func CreateRole(c *gin.Context) {
	var req struct {
		Key    string              `json:"key" binding:"required"`
		Desc   string              `json:"desc"`
		CopyOf string              `json:"copy_of"`
		Perms  map[string][]string `json:"perms"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色 key 必填"})
		return
	}
	key := strings.TrimSpace(req.Key)
	if len(key) < 2 || len(key) > 32 || strings.ContainsAny(key, " /\\") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色 key 须 2-32 位且不含空格或斜杠"})
		return
	}
	if builtinRoles[key] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "与内置角色重名"})
		return
	}
	settings := service.GetRoleSettings()
	if _, exists := settings[key]; exists {
		c.JSON(http.StatusConflict, gin.H{"error": "角色已存在"})
		return
	}

	// Base config: copy the source role or start blank (dashboard menu only)
	rp := service.RolePerm{Desc: req.Desc, Menus: []string{"dashboard"}, Perms: map[string][]string{}}
	if req.CopyOf != "" {
		if src, ok := settings[req.CopyOf]; ok {
			rp.Desc = req.Desc
			rp.Menus = append([]string{}, src.Menus...)
			rp.Perms = map[string][]string{}
			for m, acts := range src.Perms {
				rp.Perms[m] = append([]string{}, acts...)
			}
			rp.Host = src.Host
			rp.Cred, rp.Report = src.Cred, src.Report
			rp.K8sView, rp.K8sManage = src.K8sView, src.K8sManage
		}
	}
	// Perms explicitly provided by the caller take precedence (frontend matrix can submit directly)
	if len(req.Perms) > 0 {
		rp.Perms = req.Perms
	}
	if rp.Perms == nil {
		rp.Perms = map[string][]string{}
	}
	settings[key] = rp
	if err := service.SetRoleSettings(settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "ROLE", Resource: "CREATE ROLE " + key,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true, "key": key})
}

// DeleteRole DELETE /api/system/roles/:key  deletes a custom role (built-in roles forbidden; roles still referenced by users forbidden)
func DeleteRole(c *gin.Context) {
	key := c.Param("key")
	if builtinRoles[key] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内置角色不可删除"})
		return
	}
	var cnt int64
	model.DB.Model(&model.User{}).Where("role = ?", key).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("该角色仍有 %d 个用户使用，请先改派角色", cnt), "count": cnt})
		return
	}
	settings := service.GetRoleSettings()
	if _, exists := settings[key]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "角色不存在"})
		return
	}
	delete(settings, key)
	if err := service.SetRoleSettings(settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "ROLE", Resource: "DELETE ROLE " + key,
		IP: c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RoleUsers GET /api/system/roles/:key/users  lists users under the role (used for pre-delete prompts)
func RoleUsers(c *gin.Context) {
	key := c.Param("key")
	var users []model.User
	model.DB.Where("role = ?", key).Select("id, username, status").Find(&users)
	c.JSON(http.StatusOK, users)
}

// GetSystemCapabilities returns module/action declarations for the role settings matrix (rendered automatically by the frontend)
func GetSystemCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, service.CapabilitiesForFront())
}

func UpdateSystemRoles(c *gin.Context) {
	var req map[string]service.RolePerm
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	// Admin permissions are always fully enabled to prevent lockout from misconfiguration
	admin := req[model.RoleAdmin]
	admin.Host.View, admin.Host.Create, admin.Host.Edit, admin.Host.Delete = true, true, true, true
	if len(admin.Menus) == 0 {
		def := service.DefaultRoleSettings()[model.RoleAdmin]
		admin.Menus = def.Menus
	}
	admin.Desc = "全部权限，含用户/系统管理"
	req[model.RoleAdmin] = admin
	if err := service.SetRoleSettings(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetPlatformKey returns platform pairing key public key info (admin)
func GetPlatformKey(c *gin.Context) {
	k, err := service.EnsurePlatformKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"name": k.Name, "public_key": k.PublicKey, "created_at": k.CreatedAt,
		"hint": "自动配对会将该公钥写入目标机 ~/.ssh/authorized_keys",
	})
}

// TestSMTPConfig sends a test email
func TestSMTPConfig(c *gin.Context) {
	var req struct {
		To string `json:"to"`
	}
	_ = c.ShouldBindJSON(&req)
	smtpCfg := service.LoadSMTPSettings()
	to := strings.FieldsFunc(req.To, func(r rune) bool { return r == ',' || r == 10 || r == 13 || r == 59 })
	if len(to) == 0 {
		to = smtpCfg.Recipients
	}
	if len(to) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写收件邮箱"})
		return
	}
	subject := "[JNexus] SMTP 配置测试邮件"
	body := "<p>这是一封 JNexus 测试邮件，收到即表示 SMTP 配置正确。</p><p style='color:#909399;font-size:12px'>By JJ Zhang Version 1.0</p>"
	if err := service.SendMail(smtpCfg, to, subject, body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// TestLDAPConfig tests LDAP connectivity with the currently saved config
func TestLDAPConfig(c *gin.Context) {
	if err := service.TestLDAP(service.LoadLDAPSettings()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// SystemInfo public endpoint: login page shows system name/version/author
func SystemInfo(c *gin.Context) {
	m := service.SystemConfigMap()
	c.JSON(http.StatusOK, gin.H{
		"system_name": m["system_name"],
		"version":     buildinfo.Get(),
		"author":      "JJ Zhang",
	})
}

// Dashboard post-login home page stats
func Dashboard(c *gin.Context) {
	u := currentUser(c)
	count := func(dst any, where string, args ...any) int64 {
		var n int64
		q := model.DB.Model(dst)
		if where != "" {
			q = q.Where(where, args...)
		}
		q.Count(&n)
		return n
	}
	online := count(&model.Host{}, "status = ?", "online")
	// Task count scope: admins/auditors see all; others count only tasks they started
	taskQ := model.DB.Model(&model.Task{})
	if !u.IsAdmin() && u.Role != model.RoleAuditor {
		taskQ = taskQ.Where("operator = ?", u.Username)
	}
	var taskCnt int64
	taskQ.Count(&taskCnt)

	// Danger rules/users are admin-scope; non-admins get 0 (frontend hides the corresponding cards)
	dangerCnt, userCnt := count(&model.DangerRule{}, "enabled = ?", true), count(&model.User{}, "")
	if !u.IsAdmin() {
		dangerCnt, userCnt = 0, 0
	}
	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"username": u.Username, "role": u.Role, "auth_source": u.AuthSource,
			"last_login_at": u.LastLoginAt,
		},
		"hosts_total":  count(&model.Host{}, ""),
		"hosts_online": online,
		"host_groups":  count(&model.HostGroup{}, ""),
		"users":        userCnt,
		"tasks":        taskCnt,
		"scripts":      count(&model.Script{}, ""),
		"apps":         count(&model.Application{}, ""),
		"releases":     count(&model.Release{}, ""),
		"danger_rules": dangerCnt,
	})
}

// ---- Password rotation: eligible account list + rotate all now ----

// RotationAccounts GET /api/system/rotation/accounts — eligible accounts (password auth, non-LDAP) + due status
func RotationAccounts(c *gin.Context) {
	type row struct {
		ID            uint       `json:"id"`
		Host          string     `json:"host"`
		Username      string     `json:"username"`
		RotateEnabled bool       `json:"rotate_enabled"`
		Days          int        `json:"days"`
		Due           bool       `json:"due"`
		LastRotated   *time.Time `json:"last_rotated_at"`
		LastResult    string     `json:"last_rotation_result"`
	}
	var creds []model.HostCredential
	// rotatable = a stored (encrypted) password exists: password-auth accounts and
	// paired key accounts whose password was kept; LDAP/domain accounts are excluded
	model.DB.Where("is_ldap = ? AND password <> ''", false).
		Order("host_id, username").Find(&creds)

	hosts := map[uint]string{}
	var hl []model.Host
	model.DB.Select("id", "name", "ip").Find(&hl)
	for _, h := range hl {
		n := h.Name
		if n == "" {
			n = h.IP
		}
		hosts[h.ID] = n
	}

	policy := service.GetRotationPolicy()
	now := time.Now()
	out := make([]row, 0, len(creds))
	for _, cr := range creds {
		days := cr.RotateDays
		if days <= 0 {
			days = policy.Days
		}
		due := cr.LastRotatedAt == nil || now.Sub(*cr.LastRotatedAt) > time.Duration(days)*24*time.Hour
		out = append(out, row{
			ID: cr.ID, Host: hosts[cr.HostID], Username: cr.Username,
			RotateEnabled: cr.RotateEnabled, Days: days, Due: due,
			LastRotated: cr.LastRotatedAt, LastResult: cr.LastRotationResult,
		})
	}
	c.JSON(http.StatusOK, gin.H{"accounts": out, "policy_days": policy.Days})
}

// RotationRunNow POST /api/system/rotation/run-now — rotates all eligible accounts immediately (reuses the async batch; frontend polls progress)
func RotationRunNow(c *gin.Context) {
	u := currentUser(c)
	var creds []model.HostCredential
	model.DB.Where("is_ldap = ? AND password <> ''", false).Find(&creds)
	ids := make([]uint, 0, len(creds))
	for _, cr := range creds {
		ids = append(ids, cr.ID)
	}
	if len(ids) == 0 {
		c.JSON(http.StatusOK, gin.H{"batch": "", "total": 0})
		return
	}
	batch := startRotationBatch(ids, u, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"batch": batch, "total": len(ids)})
}

// ---- Security watchlist (admin; see Observability admin page) ----

// SecWatchGet GET /api/system/sec/watch
func SecWatchGet(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"win":   service.SecWatchWinList(),
		"linux": service.SecWatchLinuxList(),
	})
}

// SecWatchPut PUT /api/system/sec/watch
func SecWatchPut(c *gin.Context) {
	var req struct {
		Win   []service.SecWatchWin   `json:"win"`
		Linux []service.SecWatchLinux `json:"linux"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	cleanWin := req.Win[:0]
	seen := map[int]bool{}
	for _, w := range req.Win {
		if w.ID <= 0 || seen[w.ID] {
			continue
		}
		w.Name = strings.TrimSpace(w.Name)
		if w.Name == "" {
			w.Name = fmt.Sprintf("事件 %d", w.ID)
		}
		seen[w.ID] = true
		cleanWin = append(cleanWin, w)
	}
	cleanLinux := req.Linux[:0]
	seenKw := map[string]bool{}
	for _, w := range req.Linux {
		w.KW = strings.ToLower(strings.TrimSpace(w.KW))
		if w.KW == "" || seenKw[w.KW] {
			continue
		}
		w.Name = strings.TrimSpace(w.Name)
		if w.Name == "" {
			w.Name = w.KW
		}
		seenKw[w.KW] = true
		cleanLinux = append(cleanLinux, w)
	}
	if err := service.SaveSecWatch(cleanWin, cleanLinux); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// OORetentionApply POST /api/system/oo/retention {days} — push a retention
// period to every builtin OpenObserve stream (admin). days=0 disables cleanup.
func OORetentionApply(c *gin.Context) {
	var req struct {
		Days int `json:"days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Days < 0 || req.Days > 3650 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "days 必须在 0-3650 之间（0 = 不清理）"})
		return
	}
	if err := service.SetSystemConfigs(map[string]string{"oo_retention_days": strconv.Itoa(req.Days)}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	results := service.OOApplyRetention(req.Days)
	c.JSON(http.StatusOK, gin.H{"ok": true, "days": req.Days, "results": results})
}

// OORetentionGet GET /api/system/oo/retention — current configured days
func OORetentionGet(c *gin.Context) {
	m := service.SystemConfigMap()
	days := 0
	if n, e := strconv.Atoi(strings.TrimSpace(m["oo_retention_days"])); e == nil {
		days = n
	}
	c.JSON(http.StatusOK, gin.H{"days": days})
}

// ---- Trusted CA store (admin; outbound TLS trusts system pool + uploads) ----

// ListTrustedCAs GET /api/system/trusted-ca
func ListTrustedCAs(c *gin.Context) {
	c.JSON(http.StatusOK, service.TrustedCAList())
}

// AddTrustedCA POST /api/system/trusted-ca  {name, pem}
func AddTrustedCA(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		PEM  string `json:"pem" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（pem 必填）"})
		return
	}
	rec, err := service.AddTrustedCA(req.Name, req.PEM, currentUser(c).Username)
	code := map[bool]int{true: http.StatusOK, false: http.StatusBadRequest}[err == nil]
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "TRUSTED_CA_ADD", Resource: "/api/system/trusted-ca",
		Detail: fmt.Sprintf(`{"name":%q,"ok":%t,"err":%q}`, req.Name, err == nil, errText),
		IP:     c.ClientIP(), Status: code, CreatedAt: time.Now(),
	})
	if err != nil {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": rec.ID, "name": rec.Name, "fingerprint": rec.Fingerprint})
}

// DeleteTrustedCA DELETE /api/system/trusted-ca/:id
func DeleteTrustedCA(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	err := service.DeleteTrustedCA(uint(id))
	errText := ""
	if err != nil {
		errText = err.Error()
	}
	model.DB.Create(&model.AuditLog{
		UserID: currentUser(c).ID, Username: currentUser(c).Username,
		Action: "TRUSTED_CA_DEL", Resource: "/api/system/trusted-ca/" + strconv.Itoa(id),
		Detail: fmt.Sprintf(`{"ok":%t,"err":%q}`, err == nil, errText),
		IP:     c.ClientIP(), Status: map[bool]int{true: 200, false: 400}[err == nil], CreatedAt: time.Now(),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
