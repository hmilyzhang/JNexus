// JNexus Ops Platform — By JJ Zhang, Version 1.0

package handler

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
)

// ---- Host groups ----

func ListGroups(c *gin.Context) {
	var groups []model.HostGroup
	model.DB.Find(&groups)
	// Attach the host count for each group
	type groupCnt struct {
		GroupID *uint `json:"group_id"`
		Cnt     int64 `json:"cnt"`
	}
	var cnts []groupCnt
	model.DB.Model(&model.Host{}).Select("group_id, count(*) as cnt").
		Where("group_id IS NOT NULL").Group("group_id").Scan(&cnts)
	direct := map[uint]int64{}
	for _, x := range cnts {
		if x.GroupID != nil {
			direct[*x.GroupID] = x.Cnt
		}
	}
	// Total host count includes descendant groups (multi-level tree)
	total := map[uint]int64{}
	for _, g := range groups {
		for _, id := range service.GroupAndDescendants(g.ID) {
			total[g.ID] += direct[id]
		}
	}
	out := make([]gin.H, 0, len(groups))
	for _, g := range groups {
		out = append(out, gin.H{
			"id": g.ID, "name": g.Name, "parent_id": g.ParentID,
			"description": g.Description,
			"host_count":  total[g.ID], "created_at": g.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

func CreateGroup(c *gin.Context) {
	var g model.HostGroup
	if err := c.ShouldBindJSON(&g); err != nil || g.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分组名不能为空"})
		return
	}
	if err := model.DB.Create(&g).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分组名已存在"})
		return
	}
	c.JSON(http.StatusOK, g)
}

func UpdateGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var g model.HostGroup
	if err := model.DB.First(&g, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分组不存在"})
		return
	}
	var req model.HostGroup
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.ParentID != nil {
		if *req.ParentID == g.ID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能将分组挂到自己下面"})
			return
		}
		if wouldCycle(g.ID, *req.ParentID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "不能移动到自己的后代分组下（会形成循环）"})
			return
		}
		var p model.HostGroup
		if err := model.DB.First(&p, *req.ParentID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "上级分组不存在"})
			return
		}
	}
	model.DB.Model(&g).Updates(map[string]any{"name": req.Name, "description": req.Description, "parent_id": req.ParentID})
	c.JSON(http.StatusOK, gin.H{"id": g.ID, "name": req.Name, "parent_id": req.ParentID})
}

// wouldCycle checks whether attaching a group under newParent would form a cycle
func wouldCycle(groupID uint, newParentID uint) bool {
	var groups []model.HostGroup
	model.DB.Find(&groups)
	parent := map[uint]*uint{}
	for _, g := range groups {
		parent[g.ID] = g.ParentID
	}
	cur := newParentID
	for {
		if cur == groupID {
			return true
		}
		p := parent[cur]
		if p == nil {
			return false
		}
		cur = *p
	}
}

func DeleteGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cnt int64
	model.DB.Model(&model.Host{}).Where("group_id = ?", id).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("分组下还有 %d 台主机，请先移出", cnt)})
		return
	}
	var childCnt int64
	model.DB.Model(&model.HostGroup{}).Where("parent_id = ?", id).Count(&childCnt)
	if childCnt > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分组下还有子分组，请先删除或移出"})
		return
	}
	model.DB.Delete(&model.HostGroup{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---- Hosts ----

func ListHosts(c *gin.Context) {
	var hosts []model.Host
	q := model.DB.Preload("Group").Preload("SSHKey")
	if gid := c.Query("group_id"); gid != "" {
		q = q.Where("group_id = ?", gid)
	}
	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name ILIKE ? OR ip ILIKE ?", like, like)
	}
	q.Order("id").Find(&hosts)
	// Data-level visibility: group members with restrict_visibility enabled only see hosts bound to their group
	hosts = service.HostVisibilityFilter(currentUser(c), hosts)
	c.JSON(http.StatusOK, hosts)
}

type hostReq struct {
	Name          string `json:"name"`
	IP            string `json:"ip" binding:"required"`
	Port          int    `json:"port"`
	OSType        string `json:"os_type"`        // linux / windows
	WinRMPort     int    `json:"winrm_port"`     // Windows: 5985/5986
	RDPPort       int    `json:"rdp_port"`       // Windows: 3389
	WinRMKerberos bool   `json:"winrm_kerberos"` // Windows: use Kerberos (domain) auth
	WinRMSPN      string `json:"winrm_spn"`      // Windows: SPN override, default WSMAN/<name>
	Username      string `json:"username"`       // may come from a credential template; validated uniformly in CreateHost
	AuthType      string `json:"auth_type"`
	SSHKeyID      *uint  `json:"ssh_key_id"`
	Password      string `json:"password"`
	GroupID       *uint  `json:"group_id"`
	CredLabel     string `json:"credential_label"` // purpose label for the generated OS account
	AutoPair      bool   `json:"auto_pair"`        // auto-pair keys after creating with a password
	TemplateID    *uint  `json:"template_id"`      // credential template: when set, ignores the manually entered password and uses the template username/password
}

func (r *hostReq) toHost(h *model.Host) error {
	h.Name = r.Name
	if h.Name == "" {
		h.Name = r.IP
	}
	h.IP = r.IP
	if r.Port == 0 {
		r.Port = 22
	}
	h.Port = r.Port
	h.Username = r.Username
	if r.OSType == "" {
		r.OSType = "linux"
	}
	h.OSType = r.OSType
	if r.WinRMPort == 0 && r.OSType == "windows" {
		r.WinRMPort = 5985
	}
	if r.RDPPort == 0 && r.OSType == "windows" {
		r.RDPPort = 3389
	}
	h.WinRMPort = r.WinRMPort
	h.RDPPort = r.RDPPort
	h.WinRMKerberos = r.WinRMKerberos && r.OSType == "windows"
	h.WinRMSPN = r.WinRMSPN
	h.AuthType = r.AuthType
	if h.AuthType == "" {
		h.AuthType = "key"
	}
	h.SSHKeyID = r.SSHKeyID
	h.GroupID = r.GroupID
	if r.Password != "" {
		enc, err := pkg.Encrypt(r.Password)
		if err != nil {
			return err
		}
		h.Password = enc
	}
	return nil
}

func CreateHost(c *gin.Context) {
	var req hostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（IP、用户名必填）"})
		return
	}
	// Credential template: fills the username; the template password is used only
	// when no manual password was typed (a manual password always wins)
	if req.TemplateID != nil && *req.TemplateID > 0 {
		tu, tp, isLDAP, terr := resolveTemplatePassword(*req.TemplateID)
		if terr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": terr.Error()})
			return
		}
		req.Username = tu
		if req.AuthType == "" {
			req.AuthType = "password"
		}
		if strings.TrimSpace(req.Password) == "" {
			req.Password = tp
		}
		_ = isLDAP
	}
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "err.needUser"})
		return
	}
	// Duplicate guard: reject when the same IP+port+login user already exists (safeguard against double clicks/re-submissions)
	var dup model.Host
	if err := model.DB.Where("ip = ? AND port = ? AND username = ?", req.IP, req.Port, req.Username).First(&dup).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "err.hostExists", "host": gin.H{"id": dup.ID, "name": dup.Name}})
		return
	}
	var h model.Host
	if err := req.toHost(&h); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := model.DB.Create(&h).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "创建失败"})
		return
	}

	resp := gin.H{}
	// Password auth with auto-pair requested: push the public key via the platform key; on success keep only the key credential, on failure fall back to the password credential
	if h.AuthType == "password" && req.Password != "" {
		label := strings.TrimSpace(req.CredLabel)
		if label == "" {
			label = "默认"
		}
		if _, _, perr := service.PairAndCreateCredential(&h, h.Username, req.Password, label, true); perr == nil {
			sshKey := platformKeyID()
			model.DB.Model(&h).Updates(map[string]any{"auth_type": "key", "ssh_key_id": sshKey})
			h.AuthType = "key"
			h.SSHKeyID = sshKey
			resp["paired"] = true
		} else {
			encPwd := h.Password
			cred := model.HostCredential{
				HostID: h.ID, Username: h.Username, AuthType: "password",
				Password: encPwd, Label: label, IsDefault: true,
			}
			if err := model.DB.Create(&cred).Error; err != nil {
				resp["cred_error"] = "创建 OS 账号失败: " + err.Error()
			}
			if req.AutoPair {
				resp["pair_error"] = "密钥配对失败(" + perr.Error() + ")，已保留密码认证"
			}
		}
	}
	resp["host"] = h
	c.JSON(http.StatusOK, resp)
}

// platformKeyID platform key ID (fills the host default key after successful pairing)
func platformKeyID() *uint {
	k, err := service.EnsurePlatformKey()
	if err != nil {
		return nil
	}
	id := k.ID
	return &id
}

func UpdateHost(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var h model.Host
	if err := model.DB.First(&h, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "主机不存在"})
		return
	}
	var req hostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := req.toHost(&h); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	model.DB.Save(&h)
	c.JSON(http.StatusOK, h)
}

func DeleteHost(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cnt int64
	model.DB.Model(&model.AppHost{}).Where("host_id = ?", id).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该主机被应用部署配置引用，请先在应用管理中移除"})
		return
	}
	if err := model.DB.Delete(&model.Host{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// BatchDeleteHosts batch-deletes hosts: reuses the single-delete reference check per host, returns failure details
func BatchDeleteHosts(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ids 必填"})
		return
	}
	deleted, failed := []uint{}, map[string]string{}
	for _, id := range req.IDs {
		var cnt int64
		model.DB.Model(&model.AppHost{}).Where("host_id = ?", id).Count(&cnt)
		if cnt > 0 {
			failed[fmt.Sprint(id)] = "被应用部署配置引用"
			continue
		}
		if err := model.DB.Delete(&model.Host{}, id).Error; err != nil {
			failed[fmt.Sprint(id)] = err.Error()
			continue
		}
		deleted = append(deleted, id)
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "failed": failed})
}

func sshKeyIDOf(k *model.SSHKey) *uint { id := k.ID; return &id }

// ensureGroupPath creates groups level by level along a / separated group path (reusing existing ones); returns the leaf group ID
func ensureGroupPath(path string) (*uint, error) {
	var parentID *uint
	for _, seg := range strings.Split(path, "/") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		var g model.HostGroup
		q := model.DB.Where("name = ?", seg)
		if parentID != nil {
			q = q.Where("parent_id = ?", *parentID)
		} else {
			q = q.Where("parent_id IS NULL")
		}
		err := q.First(&g).Error
		if err != nil {
			g = model.HostGroup{Name: seg, ParentID: parentID}
			if err := model.DB.Create(&g).Error; err != nil {
				return nil, fmt.Errorf("创建分组 %s 失败", seg)
			}
		}
		id := g.ID
		parentID = &id
	}
	return parentID, nil
}

// createDefaultCred creates a default OS account for an imported host (matching the host's own account)
func createDefaultCred(hostID uint, username, authType string, sshKeyID *uint, encPassword, label string) {
	if strings.TrimSpace(label) == "" {
		label = "默认"
	}
	var cnt int64
	model.DB.Model(&model.HostCredential{}).Where("host_id = ?", hostID).Count(&cnt)
	if cnt > 0 {
		return
	}
	model.DB.Create(&model.HostCredential{
		HostID: hostID, Username: username, AuthType: authType,
		SSHKeyID: sshKeyID, Password: encPassword, Label: label, IsDefault: true,
	})
}

// ImportHosts batch import: each line is name,IP,port,username,group (recommended, name required)
// Legacy format compatible: when the first column is an IP, parse in the legacy format (ip,port,username[,password],group); name defaults to the IP
// The page-level shared password goes in the request fields (not written into CSV; JSON transport avoids shell escaping),
// When password (shared or per-line) is provided and auto_pair is enabled: auto-generate a key pair and push the public key, switching to key auth on success
var ipv4Re = regexp.MustCompile(`^\d{1,3}(\.\d{1,3}){3}$`)

func ImportHosts(c *gin.Context) {
	var req struct {
		Content    string `json:"content" binding:"required"`
		SSHKeyID   *uint  `json:"ssh_key_id"`
		AuthType   string `json:"auth_type"`
		Username   string `json:"username"`         // can serve as the default username
		Password   string `json:"password"`         // page-level shared password (used as-is, no trim/escaping)
		CredLabel  string `json:"credential_label"` // label for the generated OS accounts
		AutoPair   bool   `json:"auto_pair"`
		TemplateID *uint  `json:"template_id"` // credential template: overrides the shared username/password
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.AuthType == "" {
		req.AuthType = "key"
	}
	if req.Username == "" {
		req.Username = "root"
	}
	// Credential template: injects the template username for the whole batch; the
	// template password applies only when no page-level shared password was typed
	if req.TemplateID != nil && *req.TemplateID > 0 {
		tu, tp, _, terr := resolveTemplatePassword(*req.TemplateID)
		if terr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": terr.Error()})
			return
		}
		req.Username = tu
		if strings.TrimSpace(req.Password) == "" {
			req.Password = tp
		}
		req.AuthType = "password"
	}

	// Auto-pair mode: the whole batch shares one key pair
	credLabel := strings.TrimSpace(req.CredLabel)
	if credLabel == "" {
		credLabel = "默认"
	}
	if _, err := service.EnsurePlatformKey(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "初始化平台密钥失败: " + err.Error()})
		return
	}

	var created, skipped, paired, pairFailed int
	var errors []string
	for _, line := range strings.Split(req.Content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		// First column is not an IPv4 → new format: name,IP,...
		hostName := ""
		if len(parts) >= 2 && !ipv4Re.MatchString(strings.TrimSpace(parts[0])) {
			hostName = strings.TrimSpace(parts[0])
			parts = parts[1:]
			if hostName == "" {
				errors = append(errors, "存在空名称行，请填写主机名称")
				continue
			}
		}
		ip := strings.TrimSpace(parts[0])
		if ip == "" {
			continue
		}
		if hostName == "" {
			hostName = ip // legacy format fallback
		}
		port := 22
		username := req.Username
		password := ""
		groupName := ""
		// Parse by field count to avoid ambiguity (already shifted when the optional name column is present):
		// 1 field: ip  2: ip,port  3: ip,port,user  4: ip,port,user,group  5: ip,port,user,password,group
		switch len(parts) {
		case 1:
			// IP only; everything else uses defaults
		case 2:
			if v, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				port = v
			} else {
				groupName = strings.TrimSpace(parts[1])
			}
		case 3:
			port, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			if strings.TrimSpace(parts[2]) != "" {
				username = strings.TrimSpace(parts[2])
			}
		case 4:
			port, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			if strings.TrimSpace(parts[2]) != "" {
				username = strings.TrimSpace(parts[2])
			}
			groupName = strings.TrimSpace(parts[3])
		default: // 5 fields or more
			port, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			if strings.TrimSpace(parts[2]) != "" {
				username = strings.TrimSpace(parts[2])
			}
			password = strings.TrimSpace(parts[3])
			groupName = strings.TrimSpace(parts[4])
		}

		var cnt int64
		model.DB.Model(&model.Host{}).Where("ip = ? AND port = ?", ip, port).Count(&cnt)
		if cnt > 0 {
			skipped++
			continue
		}
		var groupID *uint
		if groupName != "" {
			// Supports / separated multi-level group paths, e.g. production/database
			gid, gerr := ensureGroupPath(groupName)
			if gerr != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", ip, gerr))
				continue
			}
			groupID = gid
		}

		// Password precedence: per-line password > page-level shared password
		effectivePassword := password
		if effectivePassword == "" {
			effectivePassword = req.Password
		}

		h := model.Host{Name: hostName, IP: ip, Port: port, Username: username,
			AuthType: req.AuthType, SSHKeyID: req.SSHKeyID, GroupID: groupID, Status: "unknown"}

		// Password + auto-pair: create host with password → push public key via platform key → keep only the key credential; on failure fall back to the password credential
		if effectivePassword != "" && req.AutoPair {
			h.AuthType = "password"
			enc, err := pkg.Encrypt(effectivePassword)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", ip, err))
				continue
			}
			h.Password = enc
			if err := model.DB.Create(&h).Error; err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", ip, err))
				continue
			}
			_, _, perr := service.PairAndCreateCredential(&h, username, effectivePassword, credLabel, true)
			if perr != nil {
				pairFailed++
				createDefaultCred(h.ID, username, "password", nil, enc, credLabel)
				errors = append(errors, fmt.Sprintf("%s: 密钥配对失败(%v)，已保留密码认证", ip, perr))
				created++
				continue
			}
			paired++
			model.DB.Model(&h).Updates(map[string]any{"auth_type": "key", "ssh_key_id": platformKeyID()})
			paired++
			created++
			continue
		}

		// Password auth (no pairing)
		if effectivePassword != "" && req.AuthType == "password" {
			enc, err := pkg.Encrypt(effectivePassword)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", ip, err))
				continue
			}
			h.Password = enc
		}
		if err := model.DB.Create(&h).Error; err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", ip, err))
			continue
		}
		createDefaultCred(h.ID, h.Username, h.AuthType, h.SSHKeyID, h.Password, req.CredLabel)
		created++
	}
	c.JSON(http.StatusOK, gin.H{
		"created": created, "skipped": skipped, "errors": errors,
		"paired": paired, "pair_failed": pairFailed,
	})
}

// ProbeHostsHandler probes hosts concurrently
func ProbeHostsHandler(c *gin.Context) {
	var req struct {
		HostIDs []uint `json:"host_ids"`
	}
	_ = c.ShouldBindJSON(&req)
	online := service.ProbeHosts(req.HostIDs)
	c.JSON(http.StatusOK, gin.H{"online": online})
}

// ---- SSH keys ----

func ListKeys(c *gin.Context) {
	var keys []model.SSHKey
	model.DB.Select("id, name, public_key, created_at").Find(&keys)
	c.JSON(http.StatusOK, keys)
}

func CreateKey(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		PublicKey  string `json:"public_key"`
		PrivateKey string `json:"private_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "私钥内容必填"})
		return
	}
	enc, err := pkg.Encrypt(req.PrivateKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加密失败: " + err.Error()})
		return
	}
	k := model.SSHKey{Name: req.Name, PublicKey: strings.TrimSpace(req.PublicKey), PrivateKey: enc}
	if err := model.DB.Create(&k).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "密钥名已存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": k.ID, "name": k.Name})
}

func DeleteKey(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cnt int64
	model.DB.Model(&model.Host{}).Where("ssh_key_id = ?", id).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("有 %d 台主机正在使用该密钥", cnt)})
		return
	}
	model.DB.Delete(&model.SSHKey{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---- Task queries ----

func ListTasks(c *gin.Context) {
	u := currentUser(c)
	var tasks []model.Task
	q := model.DB
	// Scope: admins/auditors see all, others only their own tasks
	if !u.IsAdmin() && u.Role != model.RoleAuditor {
		q = q.Where("operator = ?", u.Username)
	}
	if t := c.Query("type"); t != "" {
		q = q.Where("type = ?", t)
	}
	q.Order("id DESC").Limit(100).Find(&tasks)
	c.JSON(http.StatusOK, tasks)
}

func GetTask(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	u := currentUser(c)
	var task model.Task
	if err := model.DB.First(&task, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	// Scope: non-admin/auditor users can only view their own tasks
	if !u.IsAdmin() && u.Role != model.RoleAuditor && task.Operator != u.Username {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能查看本人发起的任务"})
		return
	}
	var results []model.TaskHostResult
	model.DB.Where("task_id = ?", id).Order("id").Find(&results)
	c.JSON(http.StatusOK, gin.H{"task": task, "results": results})
}
