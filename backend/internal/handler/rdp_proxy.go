// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

// Same-origin WebSocket proxy for the RDP gateway: the browser connects to
// /rdp-gw on the JNexus origin (so HTTPS deployments need no extra reverse-proxy
// rule or exposed gateway port), and JNexus pipes the Guacamole session to the
// rdp-gateway container. Security model is unchanged: the one-time encrypted
// query token (q=...) issued by the token endpoint stays the only key material.

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var rdpGwUpgrader = websocket.Upgrader{
	// echo the guacamole subprotocol: guacamole-common-js requests it and
	// browsers abort the handshake when the server selects none
	Subprotocols: []string{"guacamole"},
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

	dialer := &websocket.Dialer{HandshakeTimeout: 10 * time.Second, Subprotocols: []string{"guacamole"}}
	gwConn, resp, err := dialer.Dial("ws://"+rdpGwTarget()+"/?"+c.Request.URL.RawQuery, nil)
	if err != nil {
		msg := err.Error()
		if resp != nil {
			msg += fmt.Sprintf(" (gateway HTTP %d)", resp.StatusCode)
		}
		fmt.Println("[rdp-gw] dial failed:", msg)
		clientConn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "gateway unreachable: "+msg),
			time.Now().Add(time.Second))
		return
	}
	fmt.Println("[rdp-gw] gateway dialed ok")
	defer gwConn.Close()

	done := make(chan struct{}, 2)
	closeReason := func(side string, err error) {
		if e, ok := err.(*websocket.CloseError); ok {
			fmt.Printf("[rdp-gw] %s closed: code=%d reason=%q\n", side, e.Code, e.Text)
			return
		}
		fmt.Printf("[rdp-gw] %s read error: %v\n", side, err)
	}
	// client → gateway
	go func() {
		defer func() { recover() }()
		for {
			mt, data, err := clientConn.ReadMessage()
			if err != nil {
				closeReason("client", err)
				close(done)
				return
			}
			if err := gwConn.WriteMessage(mt, data); err != nil {
				fmt.Printf("[rdp-gw] gateway write failed: %v\n", err)
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
				closeReason("gateway", err)
				close(done)
				return
			}
			if err := clientConn.WriteMessage(mt, data); err != nil {
				fmt.Printf("[rdp-gw] client write failed: %v\n", err)
				close(done)
				return
			}
		}
	}()
	<-done
}
