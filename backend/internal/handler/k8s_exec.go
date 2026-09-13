// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

import (
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
)

// K8S pod exec WebSocket terminal: relays browser WS ↔ cluster API WSS (v4.channel.k8s.io)

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
	// Permission: cluster member with user/admin role (viewer is read-only and cannot enter the terminal)
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

	// Dial the cluster API exec WebSocket
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
		log.Printf("[k8s-exec] cluster %d dial failed: %v", cl.ID, err)
		bws.WriteMessage(websocket.TextMessage, []byte("exec 连接失败: "+err.Error()))
		return
	}
	defer kconn.Close()
	log.Printf("[k8s-exec] cluster %d exec open: %s/%s (tty)", cl.ID, ns, pod)

	done := make(chan struct{})
	// Cluster → browser (v4 frames: [channel][data]; forwards stdout/stderr/error)
	go func() {
		defer close(done)
		for {
			_, data, err := kconn.ReadMessage()
			if err != nil {
				// When apiserver closes with a reason (e.g. image has no shell), pass it through to the terminal
				if ce, ok := err.(*websocket.CloseError); ok && ce.Text != "" {
					log.Printf("[k8s-exec] cluster %d exec closed: %s", cl.ID, ce.Text)
					bws.WriteMessage(websocket.TextMessage, []byte("[exec] "+ce.Text))
				} else {
					log.Printf("[k8s-exec] cluster %d stream error: %v", cl.ID, err)
				}
				return
			}
			if len(data) == 0 {
				continue
			}
			ch := data[0]
			if ch == 1 || ch == 2 {
				bws.WriteMessage(websocket.BinaryMessage, data[1:])
			} else if ch == 3 {
				// error channel: apiserver failure reason (e.g. executable file not found)
				bws.WriteMessage(websocket.TextMessage, data[1:])
			}
		}
	}()
	// Browser → cluster: the frontend already packs v4 frames ([0]=stdin / [4]=resize); pass through as-is
	for {
		_, msg, err := bws.ReadMessage()
		if err != nil {
			kconn.Close()
			return
		}
		if err := kconn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
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
