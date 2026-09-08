// JNexus 运维平台 — By JJ Zhang, Version 1.0

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

// ---- 主机分组 ----

func ListGroups(c *gin.Context) {
	var groups []model.HostGroup
	model.DB.Find(&groups)
	// 附带每组主机数量
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
	// 主机总数含后代分组（多级树）
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

// wouldCycle 检查把 group 挂到 newParent 下是否形成环
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

// ---- 主机 ----

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
	// 数据级可见性：开启 restrict_visibility 的组成员仅见本组绑定的主机
	hosts = service.HostVisibilityFilter(currentUser(c), hosts)
	c.JSON(http.StatusOK, hosts)
}

type hostReq struct {
	Name       string `json:"name"`
	IP         string `json:"ip" binding:"required"`
	Port       int    `json:"port"`
	OSType     string `json:"os_type"`     // linux / windows
	WinRMPort  int    `json:"winrm_port"`  // Windows: 5985/5986
	RDPPort    int    `json:"rdp_port"`    // Windows: 3389
	Username   string `json:"username"`    // 可由凭据模板提供，CreateHost 内统一校验
	AuthType   string `json:"auth_type"`
	SSHKeyID   *uint  `json:"ssh_key_id"`
	Password   string `json:"password"`
	GroupID    *uint  `json:"group_id"`
	CredLabel  string `json:"credential_label"` // 生成 OS 账号的用途标签
	AutoPair   bool   `json:"auto_pair"`        // 密码创建后自动配对密钥
	TemplateID *uint  `json:"template_id"`      // 凭据模板：选用后忽略手输密码，取模板用户名/密码
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
	// 凭据模板：把模板的用户名/密码注入请求（优先于手输）
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
		req.Password = tp
		_ = isLDAP
	}
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "err.needUser"})
		return
	}
	// 防重复添加：同 IP+端口+登录用户 已存在时拒绝（连点/重复提交兜底）
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
	// 密码认证且要求自动配对：用平台密钥推送公钥，成功仅保留密钥凭据；失败回退密码凭据
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

// platformKeyID 平台密钥 ID（配对成功后回填主机默认密钥）
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

// BatchDeleteHosts 批量删除主机：逐台复用单删的引用校验，返回失败明细
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

// ensureGroupPath 按 / 分隔的分组路径逐级创建分组（存在则复用），返回末级分组 ID
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

// createDefaultCred 为导入的主机生成默认 OS 账号（与主机自带账号一致）
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

// ImportHosts 批量导入：每行 名称,IP,端口,用户名,分组名（推荐，名称必填）
// 兼容旧格式：首列为 IP 时按 旧格式 解析（ip,port,username[,password],group），名称默认取 IP
// 页面级统一密码放在请求字段中（不写入 CSV，JSON 传输不经 shell 转义），
// 提供 password（统一或行内）且开启 auto_pair 时：自动生成密钥对并推送公钥，成功后切换密钥认证
var ipv4Re = regexp.MustCompile(`^\d{1,3}(\.\d{1,3}){3}$`)

func ImportHosts(c *gin.Context) {
	var req struct {
		Content    string `json:"content" binding:"required"`
		SSHKeyID   *uint  `json:"ssh_key_id"`
		AuthType   string `json:"auth_type"`
		Username   string `json:"username"`         // 可作为默认用户名
		Password   string `json:"password"`         // 页面统一密码（原样使用，不做 trim/转义）
		CredLabel  string `json:"credential_label"` // 生成的 OS 账号标签
		AutoPair   bool   `json:"auto_pair"`
		TemplateID *uint  `json:"template_id"` // 凭据模板：覆盖统一用户名/密码
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
	// 凭据模板：整批注入模板用户名/密码（优先于页面统一密码）
	if req.TemplateID != nil && *req.TemplateID > 0 {
		tu, tp, _, terr := resolveTemplatePassword(*req.TemplateID)
		if terr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": terr.Error()})
			return
		}
		req.Username = tu
		req.Password = tp
		req.AuthType = "password"
	}

	// 自动配对模式：整批共用一对密钥
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
		// 首列不是 IPv4 → 新格式：名称,IP,...
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
			hostName = ip // 旧格式回退
		}
		port := 22
		username := req.Username
		password := ""
		groupName := ""
		// 按字段数解析，避免歧义（含可选名称列时已前移）：
		// 1段: ip  2段: ip,port  3段: ip,port,user  4段: ip,port,user,group  5段: ip,port,user,password,group
		switch len(parts) {
		case 1:
			// 仅 IP，其余全用默认值
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
		default: // 5 段及以上
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
			// 支持 / 分隔的多级分组路径，如 生产/数据库
			gid, gerr := ensureGroupPath(groupName)
			if gerr != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", ip, gerr))
				continue
			}
			groupID = gid
		}

		// 密码优先级：行内密码 > 页面统一密码
		effectivePassword := password
		if effectivePassword == "" {
			effectivePassword = req.Password
		}

		h := model.Host{Name: hostName, IP: ip, Port: port, Username: username,
			AuthType: req.AuthType, SSHKeyID: req.SSHKeyID, GroupID: groupID, Status: "unknown"}

		// 密码 + 自动配对：密码建主机 → 平台密钥推送公钥 → 仅保留密钥凭据；失败回退密码凭据
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

		// 密码认证（不配对）
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

// ProbeHostsHandler 并发探测
func ProbeHostsHandler(c *gin.Context) {
	var req struct {
		HostIDs []uint `json:"host_ids"`
	}
	_ = c.ShouldBindJSON(&req)
	online := service.ProbeHosts(req.HostIDs)
	c.JSON(http.StatusOK, gin.H{"online": online})
}

// ---- SSH 密钥 ----

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

// ---- 任务查询 ----

func ListTasks(c *gin.Context) {
	u := currentUser(c)
	var tasks []model.Task
	q := model.DB
	// 范围：管理员/审计员看全量，其他人仅本人任务
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
	// 范围：非管理员/审计员只能查看本人任务
	if !u.IsAdmin() && u.Role != model.RoleAuditor && task.Operator != u.Username {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能查看本人发起的任务"})
		return
	}
	var results []model.TaskHostResult
	model.DB.Where("task_id = ?", id).Order("id").Find(&results)
	c.JSON(http.StatusOK, gin.H{"task": task, "results": results})
}
