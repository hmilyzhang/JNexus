// JNexus 运维平台 — By JJ Zhang, Version 1.0

package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
	"sync"

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
	if !user.IsAdmin() && user.Role != model.RoleOps && user.Role != model.RolePublisher {
		c.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
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
