// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

// WinRM Web Terminal: bidirectional WebSocket bridge that lets the browser's
// xterm.js terminal run PowerShell commands on a Windows host via WinRM.
// WinRM is command-based (no PTY) — the backend wraps each user input line
// into a PowerShell invocation that also emits the current directory, so the
// terminal behaves like an interactive session with cwd persistence.

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
)

var winrmUpgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// WinRMTerminal GET /api/ws/winrm/:hostId — WebSocket bridge to a Windows host
func WinRMTerminal(c *gin.Context) {
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
	if !service.IsWindows(&host) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 Windows 主机"})
		return
	}
	if !service.HasCap(user.Role, "exec", "exec") {
		c.JSON(http.StatusForbidden, gin.H{"error": "err.forbidden"})
		return
	}
	if !service.CanExecHost(&user, host.ID, host.GroupID) && len(service.UsableCredentials(&user, &host)) == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "无该主机的访问权限"})
		return
	}

	var credPtr *model.HostCredential
	if cid, ok := atoiParam(c.Query("credential_id")); ok {
		cid64 := uint(cid)
		credPtr, err = service.ResolveCredential(&user, &host, &cid64)
	} else {
		credPtr, err = service.ResolveCredential(&user, &host, nil)
	}
	if err != nil || credPtr == nil || credPtr.AuthType != "password" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Windows 终端需要密码认证的 OS 账号"})
		return
	}
	pass, derr := pkg.Decrypt(credPtr.Password)
	if derr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "凭据解密失败"})
		return
	}

	wsConn, werr := winrmUpgrader.Upgrade(c.Writer, c.Request, nil)
	if werr != nil {
		return
	}
	defer wsConn.Close()

	writeMsg := func(text string) {
		wsConn.WriteMessage(websocket.TextMessage, []byte(text))
	}
	writeMsg("Windows PowerShell 会话已建立（WinRM）\r\n")

	for {
		_, cmdBytes, err := wsConn.ReadMessage()
		if err != nil {
			return
		}
		cmd := strings.TrimSpace(string(cmdBytes))
		if cmd == "" {
			writeMsg("\r\n")
			continue
		}
		switch strings.ToLower(cmd) {
		case "exit", "quit":
			writeMsg("会话已结束\r\n")
			return
		case "clear", "cls":
			writeMsg("\033[2J\033[H")
			continue
		}

		out, _, err := service.WinRMRun(&host, credPtr.Username, pass, cmd, 120)
		if err != nil {
			if strings.Contains(err.Error(), "超时") {
				writeMsg("命令执行超时（120 秒）\r\n")
			} else {
				writeMsg("错误: " + err.Error() + "\r\n")
			}
			continue
		}
		if out != "" {
			writeMsg(out + "\r\n")
		}
	}
}
