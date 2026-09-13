// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

// Same-origin WebSocket proxy for the RDP gateway: the browser connects to
// /rdp-gw on the JNexus origin (so HTTPS deployments need no extra reverse-proxy
// rule or exposed gateway port), and JNexus pipes the Guacamole session to the
// rdp-gateway container. Security model is unchanged: the one-time encrypted
// query token (q=...) issued by the token endpoint stays the only key material.

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var rdpGwUpgrader = websocket.Upgrader{
	// the connection carries the one-time encrypted token (q) as its credential;
	// origin checking is intentionally relaxed for the same reason
	CheckOrigin: func(*http.Request) bool { return true },
}

func rdpGwTarget() string {
	if v := os.Getenv("RDP_GW_TARGET"); v != "" {
		return v
	}
	return "rdp-gateway:4823" // compose service name
}

// ProxyRDPGateway GET /rdp-gw — pipes the browser WebSocket to the RDP gateway
func ProxyRDPGateway(c *gin.Context) {
	clientConn, err := rdpGwUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // Upgrade already wrote the error
	}
	defer clientConn.Close()

	dialer := &websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	gwConn, _, err := dialer.Dial("ws://"+rdpGwTarget()+"/?"+c.Request.URL.RawQuery, nil)
	if err != nil {
		clientConn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "gateway unreachable: "+err.Error()),
			time.Now().Add(time.Second))
		return
	}
	defer gwConn.Close()

	done := make(chan struct{}, 2)
	// client → gateway
	go func() {
		defer func() { recover() }()
		for {
			mt, data, err := clientConn.ReadMessage()
			if err != nil {
				close(done)
				return
			}
			if err := gwConn.WriteMessage(mt, data); err != nil {
				close(done)
				return
			}
		}
	}()
	// gateway → client
	go func() {
		defer func() { recover() }()
		for {
			mt, data, err := gwConn.ReadMessage()
			if err != nil {
				close(done)
				return
			}
			if err := clientConn.WriteMessage(mt, data); err != nil {
				close(done)
				return
			}
		}
	}()
	<-done
}
