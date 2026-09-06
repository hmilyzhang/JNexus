// JNexus 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"autoops/internal/model"
	"autoops/internal/pkg"
	"autoops/internal/service"
)

// K8S Pod exec WebSocket 终端：浏览器 WS ↔ 集群 API WSS(v4.channel.k8s.io) 中继

var k8sExecUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

type k8sExecReq struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
	Command   string `json:"command"`
}

// K8sExecWS GET /api/ws/k8s/:clusterId?namespace=&pod=&container=&command=sh&token=
func K8sExecWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		if h := c.GetHeader("Authorization"); len(h) > 7 {
			token = h[7:]
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
	clusterID, _ := strconv.Atoi(c.Param("clusterId"))
	var cl model.K8sCluster
	if err := model.DB.First(&cl, clusterID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "集群不存在"})
		return
	}
	// 权限：集群成员 user/admin 角色（viewer 只读不可进终端）
	var mm model.K8sClusterMember
	model.DB.Where("cluster_id = ? AND user_id = ?", cl.ID, user.ID).First(&mm)
	allowed := user.IsAdmin() || service.HasK8sPerm(user.Role, "manage") ||
		service.HasK8sPerm(user.Role, "manage") && false || (mm.Role == "admin" || mm.Role == "user")
	allowed = allowed || user.IsAdmin()
	if !(mm.Role == "admin" || mm.Role == "user" || user.IsAdmin() || service.HasK8sPerm(user.Role, "manage")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无该集群 Shell 权限（需成员 user/admin 角色）"})
		return
	}
	_ = allowed

	ns, pod := c.Query("namespace"), c.Query("pod")
	container := c.Query("container")
	command := c.Query("command")
	if command == "" {
		command = "sh"
	}

	bws, err := k8sExecUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer bws.Close()

	// 拨号集群 API exec WebSocket
	caPEM, certPEM, keyPEM := service.DecryptK8sCredsPair(&cl)
	tlsCfg := &tls.Config{RootCAs: service.K8sRootCAs(caPEM), MinVersion: tls.VersionTLS12}
	if certPEM != "" && keyPEM != "" {
		pair, perr := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if perr != nil {
			return
		}
		tlsCfg.Certificates = []tls.Certificate{pair}
	}
	dialer := websocket.Dialer{TLSClientConfig: tlsCfg, Subprotocols: []string{"v4.channel.k8s.io", "v3.channel.k8s.io"}, HandshakeTimeout: 15 * time.Second}
	apiURL := service.K8sExecURL(strings.TrimRight(cl.ApiServer, "/"), ns, pod, container, command)
	kconn, _, err := dialer.Dial(apiURL, nil)
	if err != nil {
		bws.WriteMessage(websocket.TextMessage, []byte("exec 连接失败: "+err.Error()))
		return
	}
	defer kconn.Close()

	done := make(chan struct{})
	// 集群 → 浏览器（v4 帧：[channel][data]，转发 stdout/stderr/error）
	go func() {
		defer close(done)
		for {
			_, data, err := kconn.ReadMessage()
			if err != nil {
				return
			}
			if len(data) == 0 {
				continue
			}
			ch := data[0]
			if ch == 1 || ch == 2 || ch == 3 {
				bws.WriteMessage(websocket.BinaryMessage, data[1:])
			}
		}
	}()
	// 浏览器 → 集群（文本 = 用户输入，前缀 stdin 通道 0）
	for {
		_, msg, err := bws.ReadMessage()
		if err != nil {
			kconn.Close()
			return
		}
		frame := append([]byte{0}, msg...)
		if err := kconn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			kconn.Close()
			return
		}
	}
	_ = done
}

var _ = json.Marshal
var _ = strconv.Itoa
var _ = strings.TrimSpace
var _ = time.Now
