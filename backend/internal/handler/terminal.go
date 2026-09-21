// JNexus Ops Platform — By JJ Zhang, Version 1.0

package handler

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
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

// compileCheck validates a regular expression
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

// WebTerminal web terminal: bidirectional WS bridge to an SSH shell
// Route: GET /api/ws/term/:hostId?token=xxx
func WebTerminal(c *gin.Context) {
	// Authentication
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
	// Data-level permission: host-level grant or an available OS account for the host (credentials linked via user groups)
	if !service.CanExecHost(&user, host.ID, host.GroupID) && len(service.UsableCredentials(&user, &host)) == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "无该主机的访问权限"})
		return
	}
	// OS account selection: specified via ?credential_id=, otherwise the host's default usable account
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
	sess.Stderr = nil // stderr is merged into stdout

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

	// WS -> SSH (text is input, JSON messages are resize)
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
// Issues a one-time connection token (valid for 5 minutes) after checking capability and data-level permissions;
// the guacamole-lite gateway exchanges this token for real RDP credentials (credentials never pass through the browser)
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
	// OS account: optional credential_id from the body picks a specific account
	// (per-team logons); absent/invalid-typed body falls back to the default account
	var body struct {
		CredentialID *uint `json:"credential_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		body.CredentialID = nil
	}
	// Get the host's usable account (explicit pick must be usable for this user)
	cred, err := service.ResolveCredential(u, &host, body.CredentialID)
	if err != nil || cred == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "无可用 OS 账号"})
		return
	}
	pass, derr := pkg.Decrypt(cred.Password)
	if derr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": derr.Error()})
		return
	}
	// Generate the guacamole-lite encrypted query string (AES-256-CBC, GW_SECRET shared key, short-lived)
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
		"gateway": gwURLForRequest(c),
	})
}

// rdpTokens connection tokens (in-process storage; sufficient for single-instance deployments)
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

// ConsumeRDPtoken guacamole-lite callback: exchanges token for credentials (one-time, discarded after use)
func ConsumeRDPtoken(c *gin.Context) {
	// Only trust the local gateway
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

// splitDomainUser splits "DOMAIN\user" into the separate domain and username that
// FreeRDP (NLA/CredSSP) requires — an empty domain with a backslashed username fails
// domain logons. UPN form (user@realm) and plain local names pass through untouched.
func splitDomainUser(user string) (domain, name string) {
	if i := strings.IndexByte(user, '\\'); i > 0 && i < len(user)-1 {
		return user[:i], user[i+1:]
	}
	return "", user
}

// buildGuacQueryString generates an encrypted connection string compatible with guacamole-lite queryEncryption
func buildGuacQueryString(ip string, port int, user, pass string) (string, error) {
	key := []byte(gwSecret())

	domain, name := splitDomainUser(user)
	// guacamole-lite expects the token plaintext as
	// {connection: {type: "rdp", settings: {...}}} (see guacamole-lite README)
	settings := map[string]any{
		"hostname":      ip,
		"port":          strconv.Itoa(port),
		"username":      name,
		"password":      pass,
		"ignore-cert":   true,
		"resize-method": "reconnect",
		"enable-drive":  false,
		"enable-audio":  false,
		"security":      "any",
		"width":         1280,
		"height":        720,
		"dpi":           96,
	}
	if domain != "" {
		settings["domain"] = domain
	}
	plaintext, err := json.Marshal(map[string]any{
		"connection": map[string]any{"type": "rdp", "settings": settings},
	})
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
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

	// token = base64(JSON({iv: base64(iv), value: base64(ciphertext)})) — guacamole-lite format
	payload := map[string]string{
		"iv":    base64.StdEncoding.EncodeToString(iv),
		"value": base64.StdEncoding.EncodeToString(out),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// gwSecret resolves the shared AES key for the guacamole-lite query string.
// Priority: GW_SECRET env → /data/gw_secret (shared volume with rdp-gateway) →
// generated once and persisted to the data dir so both sides converge on the same key.
func gwSecret() string {
	if v := os.Getenv("GW_SECRET"); v != "" {
		if len(v) > 32 {
			v = v[:32]
		}
		return v
	}
	for _, f := range []string{"/data/gw_secret", "./data/gw_secret", "data/gw_secret"} {
		if b, err := os.ReadFile(f); err == nil {
			if v := strings.TrimSpace(string(b)); len(v) >= 32 {
				return v[:32]
			}
		}
	}
	// generate & persist next to the uploads dir (same convention as other data files)
	v := strings.TrimSpace(os.Getenv("RDP_GW_SECRET")) // pre-seeded installs
	if v == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return "JnexusRdpGatewaySecretKey-123456"[:32]
		}
		v = hex.EncodeToString(b)
	}
	for _, dir := range []string{"./data", "data", "."} {
		if err := os.MkdirAll(dir, 0o755); err == nil {
			if err := os.WriteFile(dir+"/gw_secret", []byte(v), 0o600); err == nil {
				break
			}
		}
	}
	if len(v) > 32 {
		v = v[:32]
	}
	return v
}

// gwURLForRequest returns the browser-facing RDP gateway address. Default: the
// same-origin path /rdp-gw (proxied by JNexus itself — works over HTTPS with no
// extra reverse-proxy rules or exposed gateway port). RDP_GATEWAY_URL wins when
// explicitly set (direct gateway deployments).
func gwURLForRequest(c *gin.Context) string {
	if v := os.Getenv("RDP_GATEWAY_URL"); v != "" {
		return v
	}
	scheme := "ws"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "wss"
	}
	return scheme + "://" + c.Request.Host + "/rdp-gw" // no trailing slash: gin would 301 and the WS handshake cannot follow redirects
}
