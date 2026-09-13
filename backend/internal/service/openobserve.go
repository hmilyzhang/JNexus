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
	"strconv"
	"strings"
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

// ooPushAsync fire-and-forget push so collection paths never block or fail on OO hiccups;
// silently no-ops when OpenObserve is disabled
func ooPushAsync(stream string, record map[string]any) {
	if !LoadOOSettings().Enabled {
		return
	}
	go func() {
		defer func() { recover() }()
		record["_timestamp"] = time.Now().UnixMilli()
		if err := OOIngestJSON(stream, []map[string]any{record}); err != nil {
			fmt.Println("[openobserve] push", stream, "failed:", err.Error())
		}
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
