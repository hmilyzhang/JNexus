// JNexus Ops Platform — By JJ Zhang, Version 1.0

package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/service"
)

type bodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}

// Audit records all write operations to the audit log
func Audit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}
		// Read the request body to build a summary (max 1KB)
		var summary string
		if c.Request.Body != nil {
			// Read the full body then refill it, so truncation never breaks later binding (large bodies like script content)
			raw, _ := io.ReadAll(c.Request.Body)
			summary = truncateText(sanitize(string(raw)), 1024)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
		}
		bw := &bodyWriter{ResponseWriter: c.Writer, body: bytes.NewBuffer(nil)}
		c.Writer = bw

		c.Next()

		go func(user *model.User, method, path, ip string, status int, sum string) {
			service.OOPushAudit(map[string]any{
				"username": userStr(user), "method": method, "path": path,
				"ip": ip, "status": status, "summary": sum,
			})
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
			service.OOPushDBAudit(username, method, path, ip, status, sum)
		}(CurrentUser(c), c.Request.Method, c.Request.URL.Path, c.ClientIP(), c.Writer.Status(), summary)
		_ = bw.body
	}
}

// truncateText keeps only the first 1KB of the audit summary (truncation happens after refill, so requests are unaffected)
func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

// sanitize masks sensitive fields
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

func userStr(u *model.User) string {
	if u == nil {
		return ""
	}
	return u.Username
}
