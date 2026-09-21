// JNexus Ops Platform — By JJ Zhang, Version 1.0

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
)

var webAssetUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Web assets: PAM-style web application assets (URL + vaulted credentials).
// Anyone signed in may use an asset (open); only admins manage them. Every
// open/reveal is written to the audit log.

func webAssetOut(w model.WebAsset) gin.H {
	return gin.H{
		"id": w.ID, "name": w.Name, "url": w.URL, "username": w.Username,
		"description": w.Description, "creator": w.Creator, "created_at": w.CreatedAt,
		"updated_at": w.UpdatedAt, "has_password": w.Password != "",
	}
}

// ListWebAssets GET /api/webassets — all signed-in users see the assets they may open
func ListWebAssets(c *gin.Context) {
	var assets []model.WebAsset
	model.DB.Order("id ASC").Find(&assets)
	out := make([]gin.H, 0, len(assets))
	for _, w := range assets {
		out = append(out, webAssetOut(w))
	}
	c.JSON(http.StatusOK, out)
}

type webAssetReq struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Description string `json:"description"`
}

func (r *webAssetReq) valid(c *gin.Context) bool {
	if r.Name == "" || r.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "名称和 URL 必填"})
		return false
	}
	return true
}

// CreateWebAsset POST /api/webassets — admin only
func CreateWebAsset(c *gin.Context) {
	var req webAssetReq
	if err := c.ShouldBindJSON(&req); err != nil || !req.valid(c) {
		return
	}
	u := currentUser(c)
	w := model.WebAsset{
		Name: req.Name, URL: req.URL, Username: req.Username,
		Description: req.Description, Creator: u.Username,
	}
	if req.Password != "" {
		enc, err := pkg.Encrypt(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		w.Password = enc
	}
	if err := model.DB.Create(&w).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "创建失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, webAssetOut(w))
}

// UpdateWebAsset PUT /api/webassets/:id — admin only; empty password keeps the stored one
func UpdateWebAsset(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var stored model.WebAsset
	if err := model.DB.First(&stored, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "资产不存在"})
		return
	}
	var req webAssetReq
	if err := c.ShouldBindJSON(&req); err != nil || !req.valid(c) {
		return
	}
	updates := map[string]any{
		"name": req.Name, "url": req.URL, "username": req.Username,
		"description": req.Description,
	}
	if req.Password != "" {
		enc, err := pkg.Encrypt(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		updates["password"] = enc
	}
	model.DB.Model(&stored).Updates(updates)
	stored.Name, stored.URL, stored.Username, stored.Description = req.Name, req.URL, req.Username, req.Description
	c.JSON(http.StatusOK, webAssetOut(stored))
}

// DeleteWebAsset DELETE /api/webassets/:id — admin only
func DeleteWebAsset(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DB.Delete(&model.WebAsset{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetWebAsset GET /api/webassets/:id — asset info for the session page
// (password never included; there is no reveal endpoint by design — the
// headless browser fills it server-side)
func GetWebAsset(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var w model.WebAsset
	if err := model.DB.First(&w, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "资产不存在"})
		return
	}
	c.JSON(http.StatusOK, webAssetOut(w))
}

// OpenWebAsset POST /api/webassets/:id/open — signed-in users; audited.
// Returns the target URL and account for the confirm card. The password is
// never returned: the headless browser injects it server-side.
func OpenWebAsset(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var w model.WebAsset
	if err := model.DB.First(&w, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "资产不存在"})
		return
	}
	u := currentUser(c)
	model.DB.Create(&model.AuditLog{
		UserID: u.ID, Username: u.Username,
		Action: "WEBASSET_OPEN", Resource: "/api/webassets/" + strconv.Itoa(id),
		Detail: `{"name":"` + w.Name + `","account":"` + w.Username + `"}`,
		IP:     c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})
	c.JSON(http.StatusOK, gin.H{"url": w.URL, "username": w.Username})
}

// StreamWebAsset GET /api/webassets/:id/stream?token= — starts a headless-browser
// session for the asset: the server opens the page, auto-fills the vaulted
// credentials and streams the screen over this WebSocket. Input events arrive on
// the same socket. Credentials never reach the user's browser.
func StreamWebAsset(c *gin.Context) {
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
	id, _ := strconv.Atoi(c.Param("id"))
	var asset model.WebAsset
	if err := model.DB.First(&asset, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "资产不存在"})
		return
	}
	if asset.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该资产未保存密码，无法自动登录"})
		return
	}
	password, derr := pkg.Decrypt(asset.Password)
	if derr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解密失败: " + derr.Error()})
		return
	}

	wsConn, uerr := webAssetUpgrader.Upgrade(c.Writer, c.Request, nil)
	if uerr != nil {
		return
	}
	defer wsConn.Close()

	sess := service.NewWebSession(asset, asset.Username, user.Username)
	sess.SetBroadcast(func(msg map[string]any) {
		_ = wsConn.WriteJSON(msg)
	})
	model.DB.Create(&model.AuditLog{
		UserID: user.ID, Username: user.Username,
		Action: "WEBASSET_SESSION", Resource: "/api/webassets/" + strconv.Itoa(id),
		Detail: `{"name":"` + asset.Name + `","account":"` + asset.Username + `"}`,
		IP:     c.ClientIP(), Status: 200, CreatedAt: time.Now(),
	})

	go func() {
		if serr := service.RunWebSession(sess, asset.URL, asset.Username, password); serr != nil {
			sess.Send(map[string]any{"type": "error", "message": serr.Error()})
		}
	}()

	for {
		_, raw, rerr := wsConn.ReadMessage()
		if rerr != nil {
			break
		}
		var msg map[string]any
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}
		switch msg["t"] {
		case "click":
			x, y := toFloat(msg["x"]), toFloat(msg["y"])
			_ = sess.Dispatch(func(ctx context.Context) error {
				press := &input.DispatchKeyEventParams{}
				_ = press
				mp := &input.DispatchMouseEventParams{Type: input.MousePressed, X: x, Y: y, Button: input.Left, ClickCount: 1}
				if err := mp.Do(ctx); err != nil {
					return err
				}
				mr := &input.DispatchMouseEventParams{Type: input.MouseReleased, X: x, Y: y, Button: input.Left, ClickCount: 1}
				return mr.Do(ctx)
			})
		case "wheel":
			x, y, dy := toFloat(msg["x"]), toFloat(msg["y"]), toFloat(msg["dy"])
			_ = sess.Dispatch(func(ctx context.Context) error {
				ev := &input.DispatchMouseEventParams{Type: input.MouseWheel, X: x, Y: y, DeltaY: dy}
				return ev.Do(ctx)
			})
		case "text":
			if txt, _ := msg["text"].(string); txt != "" {
				_ = sess.Dispatch(func(ctx context.Context) error {
					return (&input.InsertTextParams{Text: txt}).Do(ctx)
				})
			}
		case "key":
			if key, _ := msg["key"].(string); key != "" {
				_ = sess.Dispatch(func(ctx context.Context) error {
					return dispatchSpecialKey(ctx, key)
				})
			}
		}
	}
	sess.Cancel()
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

// dispatchSpecialKey maps a UI key name to CDP key events for the session
func dispatchSpecialKey(ctx context.Context, key string) error {
	type kp struct {
		key, code string
		vk        int
	}
	spec := map[string]kp{
		"Enter": {"Enter", "Enter", 13}, "Backspace": {"Backspace", "Backspace", 8},
		"Tab": {"Tab", "Tab", 9}, "Escape": {"Escape", "Escape", 27},
		"ArrowUp": {"ArrowUp", "ArrowUp", 38}, "ArrowDown": {"ArrowDown", "ArrowDown", 40},
		"ArrowLeft": {"ArrowLeft", "ArrowLeft", 37}, "ArrowRight": {"ArrowRight", "ArrowRight", 39},
	}
	k, ok := spec[key]
	if !ok {
		return nil
	}
	for _, typ := range []input.KeyType{input.KeyDown, input.KeyUp} {
		ev := &input.DispatchKeyEventParams{Type: typ, Key: k.key, Code: k.code, WindowsVirtualKeyCode: int64(k.vk)}
		if err := ev.Do(ctx); err != nil {
			return err
		}
	}
	return nil
}
