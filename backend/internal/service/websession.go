// JNexus Ops Platform — By JJ Zhang, Version 1.0

package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/chromedp/cdproto/page"
	"jnexus/internal/model"
)

// Web session (PAM phase B): a headless Chrome opens the asset's web page,
// auto-fills the vaulted credentials and streams the screen to the user's
// browser over a WebSocket. Credentials stay on the server — the user only
// sees the streamed picture.

const (
	webSessionW      = 1280
	webSessionH      = 800
	webSessionMax    = 30 * time.Minute // hard lifetime cap
	webSessionSteady = 400 * time.Millisecond
	webLoginSettle   = 1500 * time.Millisecond
)

type WebSession struct {
	ID        string
	AssetID   uint
	Account   string
	Operator  string
	StartedAt time.Time

	mu        sync.Mutex
	browser   context.Context
	cancel    context.CancelFunc
	broadcast func(map[string]any)
	closed    bool
}

var (
	webSessionsMu sync.Mutex
	webSessions   = map[string]*WebSession{}
)

// NewWebSession registers a session; the actual browser starts in RunWebSession
func NewWebSession(asset model.WebAsset, account, operator string) *WebSession {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	s := &WebSession{
		ID:        "ws-" + hex.EncodeToString(b),
		AssetID:   asset.ID,
		Account:   account,
		Operator:  operator,
		StartedAt: time.Now(),
	}
	webSessionsMu.Lock()
	webSessions[s.ID] = s
	webSessionsMu.Unlock()
	return s
}

func (s *WebSession) SetBroadcast(fn func(map[string]any)) {
	s.mu.Lock()
	s.broadcast = fn
	s.mu.Unlock()
}

func (s *WebSession) Send(msg map[string]any) {
	s.mu.Lock()
	fn := s.broadcast
	s.mu.Unlock()
	if fn != nil {
		fn(msg)
	}
}

func (s *WebSession) Cancel() {
	s.mu.Lock()
	cancel, closed := s.cancel, s.closed
	s.mu.Unlock()
	if !closed {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
	}
	if cancel != nil {
		cancel()
	}
	webSessionsMu.Lock()
	delete(webSessions, s.ID)
	webSessionsMu.Unlock()
}

func (s *WebSession) Dispatch(fn func(context.Context) error) error {
	s.mu.Lock()
	bctx := s.browser
	s.mu.Unlock()
	if bctx == nil {
		return fmt.Errorf("会话未就绪")
	}
	ctx, cancel := context.WithTimeout(bctx, 5*time.Second)
	defer cancel()
	return chromedp.Run(ctx, chromedp.ActionFunc(fn))
}

// GetWebSession returns the live session (nil when unknown)
func GetWebSession(id string) *WebSession {
	webSessionsMu.Lock()
	defer webSessionsMu.Unlock()
	return webSessions[id]
}

// sanitizeWebURL enforces http(s) so the headless browser can't be pointed at file:// etc.
func sanitizeWebURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	return raw, nil
}

// fillAndSubmitJS builds one self-contained page-side script that fills the
// visible login form and submits it. Returns silently when no password field
// exists (already logged in / no auth page).
func fillAndSubmitJS(username, password string) string {
	uba, _ := json.Marshal(username)
	pba, _ := json.Marshal(password)
	u, p := string(uba), string(pba)
	return `(() => {
  const vis = e => e && e.offsetParent !== null;
  const pwd = [...document.querySelectorAll('input[type="password"]')].filter(vis)[0];
  if (!pwd) return 'no-auth';
  const form = pwd.closest('form');
  const scope = form || document;
  const accSels = ['input[name*="user" i]','input[id*="user" i]','input[type="email"]','input[name*="mail" i]','input[name*="login" i]','input[placeholder*="user" i]','input[placeholder*="账号" i]','input[placeholder*="用户" i]','input:not([type="password"]):not([type="hidden"]):not([type="checkbox"]):not([type="radio"])'];
  let acc = null;
  for (const sel of accSels) { acc = [...scope.querySelectorAll(sel)].filter(vis)[0]; if (acc) break; }
  const setVal = (el, v) => {
    const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    const setter = Object.getOwnPropertyDescriptor(proto, 'value').set;
    setter.call(el, v);
    el.dispatchEvent(new Event('input', { bubbles: true }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
  };
  if (acc) setVal(acc, ` + u + `);
  setVal(pwd, ` + p + `);
  const btns = [...(form || document).querySelectorAll('button, input[type="submit"]')]
    .filter(e => e.offsetParent !== null && /log ?in|sign ?in|登录|登陆|submit|确定|确认/i.test((e.textContent || '') + (e.value || '') + (e.type || '')));
  if (btns[0]) { btns[0].click(); return 'submitted'; }
  if (form && form.requestSubmit) { form.requestSubmit(); return 'form-submit'; }
  return 'filled';
})()`
}

// RunWebSession blocks until the session ends: starts headless Chrome, logs in
// with the vaulted credentials, then streams JPEG screenshots to the subscriber.
func RunWebSession(s *WebSession, rawURL, username, password string) error {
	target, err := sanitizeWebURL(rawURL)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), webSessionMax)
	s.mu.Lock()
	s.cancel = cancel
	s.browser = nil // set after browser context creation
	s.mu.Unlock()
	defer func() {
		cancel()
		s.Send(map[string]any{"type": "end"})
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		webSessionsMu.Lock()
		delete(webSessions, s.ID)
		webSessionsMu.Unlock()
	}()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(webSessionW, webSessionH+80),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()
	s.mu.Lock()
	s.browser = browserCtx
	s.mu.Unlock()

	s.Send(map[string]any{"type": "status", "stage": "opening"})

	if err := chromedp.Run(browserCtx,
		chromedp.Navigate(target),
		chromedp.Sleep(webLoginSettle),
		chromedp.Evaluate(fillAndSubmitJS(username, password), nil),
		chromedp.Sleep(1200*time.Millisecond),
	); err != nil {
		s.Send(map[string]any{"type": "error", "message": "打开页面失败: " + err.Error()})
		return err
	}
	s.Send(map[string]any{"type": "status", "stage": "live"})

	ticker := time.NewTicker(webSessionSteady)
	defer ticker.Stop()
	// first frame immediately
	frame := func() {
		var buf []byte
		if err := chromedp.Run(browserCtx, captureJPEG(60, &buf)); err == nil && len(buf) > 0 {
			s.Send(map[string]any{"type": "frame", "data": base64.StdEncoding.EncodeToString(buf)})
		}
	}
	frame()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			frame()
		}
	}
}

// captureJPEG grabs the current viewport as JPEG via the CDP screenshot command
func captureJPEG(quality int64, buf *[]byte) chromedp.QueryAction {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		params := page.CaptureScreenshot().
			WithFormat(page.CaptureScreenshotFormatJpeg).
			WithQuality(quality).
			WithCaptureBeyondViewport(false)
		v, err := params.Do(ctx)
		if err != nil {
			return err
		}
		*buf = v
		return nil
	})
}
