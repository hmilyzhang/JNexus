// JNexus 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
	"jnexus/internal/sshpool"
)

// LogTailWS 日志实时跟随：GET /api/ws/tail?token=xxx&host_id=1&path=/var/log/syslog&lines=200
// 轮询方案：每次轮询用短命 exec 查文件大小与增量（无远端常驻进程、零泄漏），
// 文件缩短视为轮转并重置。WS 断开即停止轮询。
func LogTailWS(c *gin.Context) {
	// 鉴权（与 WebTerminal 相同）
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

	hostID, ok := atoiParam(c.Query("host_id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host_id 非法"})
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
	if !service.CanExecHost(&user, host.ID, host.GroupID) && len(service.UsableCredentials(&user, &host)) == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "无该主机的访问权限"})
		return
	}
	credPtr, err := service.ResolveCredential(&user, &host, nil)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	path := strings.TrimSpace(c.Query("path"))
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path 必填"})
		return
	}
	lines := 200
	if n, e := strconv.Atoi(c.Query("lines")); e == nil && n >= 0 && n <= 5000 {
		lines = n
	}
	if lines <= 0 {
		lines = 200
	}

	ws, err := termUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()
	sendMsg := func(text string) { _ = ws.WriteMessage(websocket.TextMessage, []byte(text)) }
	onOut := sshpool.OutputWriter(func(chunk string) { _ = ws.WriteMessage(websocket.TextMessage, []byte(chunk)) })

	sendMsg("[tail] connecting " + host.Name + ": " + path)

	cli, err := sshpool.ClientForCredential(&host, credPtr)
	if err != nil {
		sendMsg("[tail] failed: " + err.Error())
		return
	}
	defer cli.Close()

	isWin := service.IsWindows(&host)
	var sizeCmd, fromCmdFmt, tailCmd string
	if isWin {
		p := strings.ReplaceAll(path, "'", "''")
		sizeCmd = fmt.Sprintf(`powershell -NoProfile -Command "(Get-Item -LiteralPath '%s').Length"`, p)
		tailCmd = fmt.Sprintf(`powershell -NoProfile -Command "Get-Content -Path '%s' -Tail %d"`, p, lines)
		fromCmdFmt = `powershell -NoProfile -Command "Get-Content -Path '%s' -ReadCount 0 | Select-Object -Skip %d"`
	} else {
		q := quotePath(path)
		sizeCmd = fmt.Sprintf("wc -c < %s", q)
		tailCmd = fmt.Sprintf("tail -n %d %s", lines, q)
		fromCmdFmt = "tail -c +%d %s"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	capture := func(cmd string, timeout time.Duration) (string, bool) {
		var sb strings.Builder
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = sshpool.RunCommand(ctx, cli, cmd, timeout, func(chunk string) { sb.WriteString(chunk) })
		}()
		select {
		case <-done:
			return sb.String(), true
		case <-time.After(timeout + 3*time.Second):
			return "", false
		}
	}

	// 初始回读：尾部 lines 行
	if out, ok := capture(tailCmd, 12*time.Second); ok {
		onOut(out)
	} else {
		sendMsg("[tail] 初始读取失败，继续跟随新内容")
	}

	// 当前大小作为轮询基准
	offset := int64(0)
	if out, ok := capture(sizeCmd, 10*time.Second); ok {
		if n, e := strconv.ParseInt(strings.TrimSpace(out), 10, 64); e == nil {
			offset = n
		}
	}

	sendMsg("[tail] following")

	// 轮询循环：2s 一次
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	wsClose := make(chan struct{})
	go func() {
		for {
			if _, _, werr := ws.ReadMessage(); werr != nil {
				break
			}
		}
		close(wsClose)
	}()

	for {
		select {
		case <-wsClose:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		var sizeStr string
		var ok bool
		if isWin {
			sizeStr, ok = capture(fmt.Sprintf(`powershell -NoProfile -Command "(Get-Item -LiteralPath '%s').Length"`, strings.ReplaceAll(path, "'", "''")), 8*time.Second)
		} else {
			sizeStr, ok = capture(fmt.Sprintf("wc -c < %s", quotePath(path)), 8*time.Second)
		}
		if !ok {
			continue
		}
		size, e := strconv.ParseInt(strings.TrimSpace(sizeStr), 10, 64)
		if e != nil || size < 0 {
			continue
		}
		if size < offset { // 轮转/截断：从头重读
			offset = 0
			sendMsg("[tail] file rotated, restarting from beginning")
		}
		if size == offset {
			continue
		}
		var readCmd string
		if isWin {
			readCmd = fmt.Sprintf(fromCmdFmt, path, offset)
		} else {
			readCmd = fmt.Sprintf(fromCmdFmt, offset+1, quotePath(path))
		}
		if out, ok := capture(readCmd, 10*time.Second); ok {
			onOut(out)
			offset = size
		}
	}
}

// quotePath POSIX 单引号包裹（内嵌单引号转义）
func quotePath(p string) string {
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}
