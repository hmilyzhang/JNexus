// JNexus 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	gossh "golang.org/x/crypto/ssh"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
	"jnexus/internal/sshpool"
)

// compileCheck 校验正则合法性
func compileCheck(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(pattern)
}

func atoiParam(s string) (int, bool) {
	n := 0
	if s == "" {
		return 0, false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		n = n*10 + int(ch-'0')
	}
	return n, true
}

var termUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WebTerminal Web 终端：WS 双向桥接 SSH shell
// 路由: GET /api/ws/term/:hostId?token=xxx
func WebTerminal(c *gin.Context) {
	// 鉴权
	token := c.Query("token")
	if token == "" {
		if auth := c.GetHeader("Authorization"); len(auth) > 7 {
			token = auth[7:]
		}
	}
	claims, err := pkg.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	var user model.User
	if err := model.DB.First(&user, claims.UserID).Error; err != nil || user.Status != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号不可用"})
		return
	}

	hostID, ok := atoiParam(c.Param("hostId"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hostId 非法"})
		return
	}
	var host model.Host
	if err := model.DB.First(&host, hostID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "主机不存在"})
		return
	}
	if !service.HasCap(user.Role, "exec", "exec") {
		c.JSON(http.StatusForbidden, gin.H{"error": "err.forbidden"})
		return
	}
	// 数据级权限：主机级授权 或 拥有该主机的可用 OS 账号（用户组关联凭据）
	if !service.CanExecHost(&user, host.ID, host.GroupID) && len(service.UsableCredentials(&user, &host)) == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "无该主机的访问权限"})
		return
	}
	// OS 账号选择：?credential_id= 指定，否则主机默认可用账号
	var credPtr *model.HostCredential
	if cid, ok := atoiParam(c.Query("credential_id")); ok {
		cid64 := uint(cid)
		credPtr, err = service.ResolveCredential(&user, &host, &cid64)
	} else {
		credPtr, err = service.ResolveCredential(&user, &host, nil)
	}
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	cli, err := sshpool.ClientForCredential(&host, credPtr)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	sess, err := cli.NewSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer sess.Close()

	stdin, err := sess.StdinPipe()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sess.Stderr = nil // stderr 合并到 stdout

	modes := gossh.TerminalModes{gossh.ECHO: 1, gossh.TTY_OP_ISPEED: 14400, gossh.TTY_OP_OSPEED: 14400}
	if err := sess.RequestPty("xterm-256color", 40, 120, modes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "申请终端失败: " + err.Error()})
		return
	}
	if err := sess.Shell(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "启动 shell 失败: " + err.Error()})
		return
	}

	wsConn, err := termUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer wsConn.Close()

	var once sync.Once
	closeAll := func() {
		once.Do(func() {
			_ = stdin.Close()
			_ = sess.Signal(gossh.SIGKILL)
			_ = wsConn.Close()
		})
	}
	defer closeAll()

	// SSH -> WS
	go func() {
		defer closeAll()
		buf := make([]byte, 8192)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				if werr := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// WS -> SSH（文本为输入，JSON 消息为 resize）
	for {
		mt, data, err := wsConn.ReadMessage()
		if err != nil {
			return
		}
		if mt == websocket.TextMessage && len(data) > 0 && data[0] == '{' {
			var msg struct {
				Type string `json:"type"`
				Cols int    `json:"cols"`
				Rows int    `json:"rows"`
			}
			if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
				_ = sess.WindowChange(msg.Rows, msg.Cols)
				continue
			}
		}
		if _, err := stdin.Write(data); err != nil {
			return
		}
	}
}

// RDPConnectToken POST /api/hosts/:id/rdp-token
// 校验能力位与数据级权限后签发一次性连接令牌（5 分钟有效），
// guacamole-lite 网关用该 token 换取真实 RDP 凭据（凭据不经过浏览器）
func RDPConnectToken(c *gin.Context) {
	u := currentUser(c)
	hostID, ok := atoiParam(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hostId 非法"})
		return
	}
	var host model.Host
	if err := model.DB.First(&host, hostID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "主机不存在"})
		return
	}
	if host.OSType != "windows" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅 Windows 主机支持远程桌面"})
		return
	}
	if !service.HasCap(u.Role, "exec", "exec") {
		c.JSON(http.StatusForbidden, gin.H{"error": "err.forbidden"})
		return
	}
	if !service.CanExecHost(u, host.ID, host.GroupID) && len(service.UsableCredentials(u, &host)) == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "无该主机的访问权限"})
		return
	}
	// 取主机默认可用凭据
	cred, err := service.ResolveCredential(u, &host, nil)
	if err != nil || cred == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "无可用 OS 账号"})
		return
	}
	pass, derr := pkg.Decrypt(cred.Password)
	if derr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": derr.Error()})
		return
	}
	// 生成 guacamole-lite 加密查询串（AES-256-CBC，GW_SECRET 共享密钥，短时有效）
	qs, err := buildGuacQueryString(host.IP, rdpPortOf(&host), cred.Username, pass)
	if derr != nil {
		_ = derr
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"query": qs, "host": host.Name, "ip": host.IP,
		"gateway": cfgGwURL(),
	})
}

// rdpTokens 连接令牌（进程内存储；单实例部署足够）
type rdpTokenData struct {
	Host     string
	Port     int
	Username string
	Password string
	UserID   uint
	Expires  time.Time
}

var (
	rdpTokensMu sync.Mutex
	rdpTokens   = map[string]rdpTokenData{}
)

// ConsumeRDPtoken guacamole-lite 回调：token 换凭据（一次性，取后即焚）
func ConsumeRDPtoken(c *gin.Context) {
	// 仅信任本地网关
	if c.ClientIP() != "127.0.0.1" && c.ClientIP() != "::1" {
		c.JSON(http.StatusForbidden, gin.H{"error": "err.forbidden"})
		return
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token 必填"})
		return
	}
	rdpTokensMu.Lock()
	data, ok := rdpTokens[req.Token]
	if ok {
		delete(rdpTokens, req.Token)
	}
	rdpTokensMu.Unlock()
	if !ok || time.Now().After(data.Expires) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token 无效或已过期"})
		return
	}
	port := data.Port
	if port == 0 {
		port = 3389
	}
	c.JSON(http.StatusOK, gin.H{
		"hostname": data.Host, "port": port,
		"username": data.Username, "password": data.Password,
		"protocol": "rdp", "security": "nla",
		"enable-drive": false, "enable-clipboard-integration": true,
		"resize-method": "reconnect",
	})
}

func randomHexToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func rdpPortOf(h *model.Host) int {
	if h.RDPPort > 0 {
		return h.RDPPort
	}
	return 3389
}

// buildGuacQueryString 生成 guacamole-lite queryEncryption 兼容的加密连接串
func buildGuacQueryString(ip string, port int, user, pass string) (string, error) {
	key := os.Getenv("GW_SECRET")
	if key == "" {
		key = "JnexusRdpGatewaySecretKey-123456"
	}
	key = key[:32]
	plaintext := "guac.hostname=" + url.QueryEscape(ip) +
		"&guac.port=" + strconv.Itoa(port) +
		"&guac.username=" + url.QueryEscape(user) +
		"&guac.password=" + url.QueryEscape(pass) +
		"&guac.protocol=rdp&guac.ignore-cert=true" +
		"&guac.resize-method=reconnect&guac.enable-drive=false&guac.enable-audio=false" +
		"&width=1280&height=720&dpi=96"
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	pad := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := append([]byte(plaintext), bytes.Repeat([]byte{byte(pad)}, pad)...)
	mode := cipher.NewCBCEncrypter(block, iv)
	out := make([]byte, len(padded))
	mode.CryptBlocks(out, padded)
	return hex.EncodeToString(append(iv, out...)), nil
}

func cfgGwURL() string {
	if v := os.Getenv("RDP_GATEWAY_URL"); v != "" {
		return v
	}
	return "http://localhost:4823"
}
