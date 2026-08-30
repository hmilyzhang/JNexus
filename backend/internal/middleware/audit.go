// AutoOps 运维平台 — By JJ Zhang, Version 1.0

package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"autoops/internal/model"
)

type bodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}

// Audit 记录所有写操作到审计日志
func Audit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}
		// 读取请求体做摘要（最多 1KB）
		var summary string
		if c.Request.Body != nil {
			raw, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1024))
			summary = sanitize(string(raw))
			c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
		}
		bw := &bodyWriter{ResponseWriter: c.Writer, body: bytes.NewBuffer(nil)}
		c.Writer = bw

		c.Next()

		go func(user *model.User, method, path, ip string, status int, sum string) {
			username := ""
			uid := uint(0)
			if user != nil {
				username = user.Username
				uid = user.ID
			}
			model.DB.Create(&model.AuditLog{
				UserID: uid, Username: username,
				Action: method, Resource: path, Detail: sum,
				IP: ip, Status: status, CreatedAt: time.Now(),
			})
		}(CurrentUser(c), c.Request.Method, c.Request.URL.Path, c.ClientIP(), c.Writer.Status(), summary)
		_ = bw.body
	}
}

// sanitize 屏蔽敏感字段
func sanitize(s string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return s
	}
	for _, k := range []string{"password", "old_password", "new_password", "content"} {
		if _, ok := m[k]; ok {
			m[k] = "***"
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return s
	}
	return string(out)
}
