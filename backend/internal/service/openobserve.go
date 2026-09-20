// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// OpenObserve client: dual-writes collected metrics/task output/alert events into an
// optional OpenObserve instance for long-term storage and full-text search (AGPL-3.0,
// invoked over HTTP only — no license impact on JNexus itself).

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type OOSettings struct {
	Enabled bool
	BaseURL string // e.g. http://openobserve:5080
	Org     string // organization, usually "default"
	Token   string // "user:password" (encoded here) or a pre-encoded Basic token
}

// LoadOOSettings reads OpenObserve settings from system config
func LoadOOSettings() OOSettings {
	m := SystemConfigMap()
	return OOSettings{
		Enabled: m["oo_enabled"] == "true",
		BaseURL: strings.TrimRight(strings.TrimSpace(m["oo_url"]), "/"),
		Org:     strings.TrimSpace(m["oo_org"]),
		Token:   strings.TrimSpace(m["oo_token"]),
	}
}

func (s OOSettings) authHeader() string {
	if s.Token == "" {
		return ""
	}
	if strings.Contains(s.Token, ":") { // user:password → encode
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(s.Token))
	}
	return "Basic " + s.Token // already encoded
}

func ooHTTP(timeout time.Duration, method, url, auth string, body []byte) (int, []byte, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, data, err
}

// ooJSON POSTs v as JSON to the given path; returns nil when disabled (best-effort callers)
func (s OOSettings) ooJSON(path string, v any, timeout time.Duration) (int, []byte, error) {
	if !s.Enabled || s.BaseURL == "" {
		return 0, nil, fmt.Errorf("OpenObserve is not enabled (System Settings → Observability)")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return 0, nil, err
	}
	return ooHTTP(timeout, http.MethodPost, s.BaseURL+path, s.authHeader(), b)
}

// OOSetStreamRetention sets a stream's data-retention period (days) inside
// OpenObserve; 0 disables automatic cleanup for that stream
// OOSetStreamRetention sets a stream's data-retention period (days) inside
// OpenObserve; 0 disables automatic cleanup for that stream. Verified against
// OO: the settings endpoint takes {"data_retention": N} in days.
func (s OOSettings) OOSetStreamRetention(stream string, days int) error {
	if s.BaseURL == "" {
		return fmt.Errorf("OpenObserve is not enabled")
	}
	b, err := json.Marshal(map[string]any{"data_retention": days})
	if err != nil {
		return err
	}
	code, data, err := ooHTTP(15*time.Second, http.MethodPut, s.BaseURL+"/api/"+s.Org+"/streams/"+stream+"/settings", s.authHeader(), b)
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("OpenObserve HTTP %d: %s", code, truncateOO(data))
	}
	return nil
}

// OOApplyRetention pushes a retention period (days) to every builtin stream.
// Returns per-stream results for the admin UI.
func OOApplyRetention(days int) []map[string]any {
	s := LoadOOSettings()
	out := []map[string]any{}
	for _, stream := range ooBuiltinStreams {
		res := map[string]any{"stream": stream, "days": days}
		if err := s.OOSetStreamRetention(stream, days); err != nil {
			msg := err.Error()
			// stream not created yet (no data ingested): friendly note
			if strings.Contains(msg, "could not be found") {
				msg = "流尚未创建（暂无数据写入），待数据产生后重新应用即可"
			}
			res["ok"] = false
			res["error"] = msg
		} else {
			res["ok"] = true
		}
		out = append(out, res)
	}
	return out
}

// OOPushAudit writes a sanitized platform-audit record to the audit stream
func OOPushAudit(record map[string]any) {
	ooPushAsync("audit", record)
}

// OOIngestJSON writes log-style records into a stream: POST /api/{org}/{stream}/_json
func OOIngestJSON(stream string, records []map[string]any) error {
	s := LoadOOSettings()
	if len(records) == 0 {
		return nil
	}
	code, data, err := s.ooJSON("/api/"+s.Org+"/"+stream+"/_json", records, 10*time.Second)
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("OpenObserve HTTP %d: %s", code, truncateOO(data))
	}
	return nil
}

// OOSearch runs a SQL query: POST /api/{org}/_search; startMs/endMs are epoch milliseconds
func OOSearch(sql string, startMs, endMs int64, from, size int) (map[string]any, error) {
	s := LoadOOSettings()
	body := map[string]any{
		"query": map[string]any{
			"sql":        sql,
			"start_time": startMs * 1000, // OO search API uses microseconds
			"end_time":   endMs * 1000,
			"from":       from,
			"size":       size,
		},
	}
	code, data, err := s.ooJSON("/api/"+s.Org+"/_search", body, 30*time.Second)
	if err != nil {
		return nil, err
	}
	if code >= 400 {
		return nil, fmt.Errorf("OpenObserve HTTP %d: %s", code, truncateOO(data))
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("failed to parse OpenObserve response")
	}
	return out, nil
}

// OOConfigTest validates the stored settings via the streams list endpoint
// (a plain "SELECT 1" search panics some OpenObserve builds, so avoid it)
func OOConfigTest() error {
	s := LoadOOSettings()
	if s.BaseURL == "" || s.Org == "" || s.Token == "" {
		return fmt.Errorf("URL, organization and credentials are required")
	}
	code, data, err := ooHTTP(10*time.Second, http.MethodGet, s.BaseURL+"/api/"+s.Org+"/streams", s.authHeader(), nil)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	if code >= 400 {
		return fmt.Errorf("OpenObserve HTTP %d: %s", code, truncateOO(data))
	}
	return nil
}

func truncateOO(b []byte) string {
	s := string(b)
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

// ---- Push stats + per-stream toggles (in-process; resets on restart by design) ----

// OOStreamStat push counters for one stream
type OOStreamStat struct {
	Pushed      int64      `json:"pushed"`
	Failed      int64      `json:"failed"`
	LastPushAt  *time.Time `json:"last_push_at"`
	LastError   string     `json:"last_error,omitempty"`
	LastErrorAt *time.Time `json:"last_error_at"`
}

var ooStatsMu sync.Mutex
var ooStats = map[string]*OOStreamStat{}

func ooStat(stream string) *OOStreamStat {
	ooStatsMu.Lock()
	defer ooStatsMu.Unlock()
	st := ooStats[stream]
	if st == nil {
		st = &OOStreamStat{}
		ooStats[stream] = st
	}
	return st
}

// ooIntegrationEnabled reads the per-stream toggle from oo_integrations config
// (JSON map; a missing entry defaults to enabled for the builtin streams)
var ooBuiltinStreams = []string{"host_metrics", "task_logs", "alert_events", "windows_events", "linux_events", "db_audit", "k8s_capacity", "audit"}

func ooIntegrationEnabled(stream string) bool {
	m := SystemConfigMap()
	cfg := map[string]bool{}
	_ = json.Unmarshal([]byte(m["oo_integrations"]), &cfg)
	if v, ok := cfg[stream]; ok {
		return v
	}
	for _, b := range ooBuiltinStreams {
		if b == stream {
			return true
		}
	}
	return false // unknown/custom streams default off until explicitly enabled
}

// OOSetIntegration persists one stream toggle into oo_integrations config
func OOSetIntegration(stream string, enabled bool) error {
	m := SystemConfigMap()
	cfg := map[string]bool{}
	_ = json.Unmarshal([]byte(m["oo_integrations"]), &cfg)
	cfg[stream] = enabled
	b, _ := json.Marshal(cfg)
	return SetSystemConfigs(map[string]string{"oo_integrations": string(b)})
}

// OOStats returns a copy of the push statistics for all streams
func OOStats() map[string]OOStreamStat {
	ooStatsMu.Lock()
	defer ooStatsMu.Unlock()
	out := make(map[string]OOStreamStat, len(ooStats))
	for k, v := range ooStats {
		out[k] = *v
	}
	return out
}

// AutoConfigureOO seeds the OpenObserve integration from ZO_ROOT_USER_EMAIL /
// ZO_ROOT_USER_PASSWORD environment variables (compose injects the same values into
// the jnexus container as into openobserve). Runs at startup; only fills in what is
// missing, so manually configured values are never overwritten.
func AutoConfigureOO() {
	email, pass := os.Getenv("ZO_ROOT_USER_EMAIL"), os.Getenv("ZO_ROOT_USER_PASSWORD")
	if email == "" || pass == "" {
		return
	}
	m := SystemConfigMap()
	if strings.TrimSpace(m["oo_token"]) != "" {
		return // already configured
	}
	url := strings.TrimSpace(m["oo_url"])
	if url == "" {
		url = "http://openobserve:5080" // compose service name
	}
	org := strings.TrimSpace(m["oo_org"])
	if org == "" {
		org = "default"
	}
	enabled := m["oo_enabled"]
	if enabled == "" {
		enabled = "true"
	}
	if err := SetSystemConfigs(map[string]string{
		"oo_enabled": enabled, "oo_url": url, "oo_org": org,
		"oo_token": email + ":" + pass,
	}); err != nil {
		fmt.Println("[openobserve] auto-config failed:", err.Error())
		return
	}
	fmt.Println("[openobserve] integration auto-configured from ZO_ROOT_USER_* environment")
}

// OOListStreams returns the stream names present in OpenObserve
func OOListStreams() ([]string, error) {
	s := LoadOOSettings()
	if !s.Enabled || s.BaseURL == "" {
		return nil, fmt.Errorf("OpenObserve is not enabled (System Settings → Observability)")
	}
	code, data, err := ooHTTP(10*time.Second, http.MethodGet, s.BaseURL+"/api/"+s.Org+"/streams", s.authHeader(), nil)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	if code >= 400 {
		return nil, fmt.Errorf("OpenObserve HTTP %d: %s", code, truncateOO(data))
	}
	var out struct {
		List []struct {
			Name string `json:"name"`
		} `json:"list"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("failed to parse streams response")
	}
	names := make([]string, 0, len(out.List))
	for _, it := range out.List {
		names = append(names, it.Name)
	}
	return names, nil
}

// OOStatus aggregates connection health + push stats for the admin page
func OOStatus() map[string]any {
	s := LoadOOSettings()
	out := map[string]any{
		"enabled":      s.Enabled,
		"url":          s.BaseURL,
		"org":          s.Org,
		"token_set":    s.Token != "",
		"integrations": SystemConfigMap()["oo_integrations"],
		"stats":        OOStats(),
	}
	if s.Enabled && s.BaseURL != "" {
		start := time.Now()
		code, _, err := ooHTTP(5*time.Second, http.MethodGet, s.BaseURL+"/api/"+s.Org+"/streams", s.authHeader(), nil)
		out["reachable"] = err == nil && code < 400
		out["latency_ms"] = time.Since(start).Milliseconds()
		if err != nil {
			out["error"] = err.Error()
		} else if code >= 400 {
			out["error"] = fmt.Sprintf("HTTP %d", code)
		}
		if names, err := OOListStreams(); err == nil {
			out["streams"] = names
		}
	} else {
		out["reachable"] = false
	}
	return out
}

var ooStreamNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

// OOStreamNameValid reports whether a custom stream name is safe to use
func OOStreamNameValid(stream string) bool { return ooStreamNameRe.MatchString(stream) }

// OOIngestCustom validates a custom stream name and pushes arbitrary records
func OOIngestCustom(stream string, records []map[string]any) (int, error) {
	if !ooStreamNameRe.MatchString(stream) {
		return 0, fmt.Errorf("invalid stream name (allowed: letters, digits, _ and -, max 100)")
	}
	if len(records) == 0 {
		return 0, fmt.Errorf("records is empty")
	}
	if err := OOIngestJSON(stream, records); err != nil {
		st := ooStat(stream)
		ooStatsMu.Lock()
		st.Failed += int64(len(records))
		now := time.Now()
		st.LastError, st.LastErrorAt = err.Error(), &now
		ooStatsMu.Unlock()
		return 0, err
	}
	st := ooStat(stream)
	ooStatsMu.Lock()
	st.Pushed += int64(len(records))
	now := time.Now()
	st.LastPushAt = &now
	ooStatsMu.Unlock()
	return len(records), nil
}

// ooPushAsync fire-and-forget push so collection paths never block or fail on OO hiccups;
// silently no-ops when OpenObserve is disabled or the stream toggle is off
func ooPushAsync(stream string, record map[string]any) {
	if !LoadOOSettings().Enabled || !ooIntegrationEnabled(stream) {
		return
	}
	go func() {
		defer func() { recover() }()
		record["_timestamp"] = time.Now().UnixMilli()
		if err := OOIngestJSON(stream, []map[string]any{record}); err != nil {
			st := ooStat(stream)
			ooStatsMu.Lock()
			st.Failed++
			now := time.Now()
			st.LastError, st.LastErrorAt = err.Error(), &now
			ooStatsMu.Unlock()
			fmt.Println("[openobserve] push", stream, "failed:", err.Error())
			return
		}
		st := ooStat(stream)
		ooStatsMu.Lock()
		st.Pushed++
		now := time.Now()
		st.LastPushAt = &now
		ooStatsMu.Unlock()
	}()
}

// OOPushHostMetric dual-writes one host metrics sample (called after the DB write)
func OOPushHostMetric(hostID uint, hostName string, cpu, mem, disk float64, at time.Time) {
	ooPushAsync("host_metrics", map[string]any{
		"host_id": hostID, "host": hostName,
		"cpu_percent":  strconv.FormatFloat(cpu, 'f', 2, 64),
		"mem_percent":  strconv.FormatFloat(mem, 'f', 2, 64),
		"disk_percent": strconv.FormatFloat(disk, 'f', 2, 64),
		"collected_at": at.Format(time.RFC3339),
	})
}

// OOPushTaskLog dual-writes one finished task result (phase 2: task output into the log store)
func OOPushTaskLog(taskID uint, taskType, operator, host, osUser, status string, exitCode int, output string) {
	ooPushAsync("task_logs", map[string]any{
		"task_id": taskID, "type": taskType, "operator": operator, "host": host,
		"os_user": osUser, "status": status, "exit_code": exitCode,
		"output": output,
	})
}

// OOPushAlertEvent dual-writes one alert event (phase 2)
func OOPushAlertEvent(kind, level, target, message string) {
	ooPushAsync("alert_events", map[string]any{
		"kind": kind, "level": level, "target": target, "message": message,
	})
}

// OOPushDBAudit dual-writes one platform audit-log entry (database audit ingestion)
func OOPushDBAudit(username, action, resource, ip string, status int, detail string) {
	ooPushAsync("db_audit", map[string]any{
		"username": username, "action": action, "resource": resource,
		"ip": ip, "status": status, "detail": detail,
	})
}
