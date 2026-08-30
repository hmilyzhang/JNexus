// AutoOps 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"autoops/internal/model"
	"autoops/internal/pkg"
	"autoops/internal/service"
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
	m := map[uint]int64{}
	for _, x := range cnts {
		if x.GroupID != nil {
			m[*x.GroupID] = x.Cnt
		}
	}
	out := make([]gin.H, 0, len(groups))
	for _, g := range groups {
		out = append(out, gin.H{
			"id": g.ID, "name": g.Name, "description": g.Description,
			"host_count": m[g.ID], "created_at": g.CreatedAt,
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
	model.DB.Model(&g).Updates(map[string]any{"name": req.Name, "description": req.Description})
	c.JSON(http.StatusOK, g)
}

func DeleteGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var cnt int64
	model.DB.Model(&model.Host{}).Where("group_id = ?", id).Count(&cnt)
	if cnt > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("分组下还有 %d 台主机，请先移出", cnt)})
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
	c.JSON(http.StatusOK, hosts)
}

type hostReq struct {
	Name     string `json:"name"`
	IP       string `json:"ip" binding:"required"`
	Port     int    `json:"port"`
	Username string `json:"username" binding:"required"`
	AuthType string `json:"auth_type"`
	SSHKeyID *uint  `json:"ssh_key_id"`
	Password string `json:"password"`
	GroupID  *uint  `json:"group_id"`
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
	var h model.Host
	if err := req.toHost(&h); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := model.DB.Create(&h).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, h)
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
		Content   string `json:"content" binding:"required"`
		SSHKeyID  *uint  `json:"ssh_key_id"`
		AuthType  string `json:"auth_type"`
		Username  string `json:"username"`  // 可作为默认用户名
		Password  string `json:"password"`  // 页面统一密码（原样使用，不做 trim/转义）
		CredLabel string `json:"credential_label"` // 生成的 OS 账号标签
		AutoPair  bool   `json:"auto_pair"`
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

	// 自动配对模式：整批共用一对密钥
	var batchKey *model.SSHKey
	credLabel := strings.TrimSpace(req.CredLabel)
	if credLabel == "" {
		credLabel = "默认"
	}
	if req.AutoPair {
		k, err := service.GenerateAndStoreKeyPair(
			fmt.Sprintf("import-%s", time.Now().Format("20060102150405")), "autoops-import")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "生成密钥对失败: " + err.Error()})
			return
		}
		batchKey = k
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
			var g model.HostGroup
			if err := model.DB.Where("name = ?", groupName).First(&g).Error; err != nil {
				g = model.HostGroup{Name: groupName}
				if err := model.DB.Create(&g).Error; err != nil {
					errors = append(errors, fmt.Sprintf("%s: 创建分组失败", ip))
					continue
				}
			}
			groupID = &g.ID
		}

		// 密码优先级：行内密码 > 页面统一密码
		effectivePassword := password
		if effectivePassword == "" {
			effectivePassword = req.Password
		}

		h := model.Host{Name: hostName, IP: ip, Port: port, Username: username,
			AuthType: req.AuthType, SSHKeyID: req.SSHKeyID, GroupID: groupID, Status: "unknown"}

		// 密码 + 自动配对：先按密码建主机，推送公钥成功后切换为密钥认证
		if effectivePassword != "" && req.AutoPair && batchKey != nil {
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
			pubLine := service.KeyPairPublicLine(batchKey)
			if err := service.InstallPubKeyWithPassword(ip, port, username, effectivePassword, pubLine); err != nil {
				pairFailed++
				errors = append(errors, fmt.Sprintf("%s: 密钥配对失败(%v)，已保留密码认证", ip, err))
				created++
				continue
			}
			model.DB.Model(&h).Updates(map[string]any{"auth_type": "key", "ssh_key_id": batchKey.ID})
			createDefaultCred(h.ID, username, "key", &batchKey.ID, "", credLabel)
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
	resp := gin.H{"created": created, "skipped": skipped, "errors": errors}
	if batchKey != nil {
		resp["key_id"] = batchKey.ID
		resp["key_name"] = batchKey.Name
		resp["paired"] = paired
		resp["pair_failed"] = pairFailed
	}
	c.JSON(http.StatusOK, resp)
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
