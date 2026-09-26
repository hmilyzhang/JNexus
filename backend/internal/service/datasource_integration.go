// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// Data-source integrations for OpenObserve: a catalog of every source type OO
// accepts (grouped like the OO UI) plus a registry of configured integrations.
// Each integration generates a ready-to-paste collector config (Fluent Bit /
// Telegraf / OpenTelemetry Collector) with the platform's OO endpoint and
// stream pre-filled — OpenObserve is a push target, so "integrating a source"
// means deploying a collector that ships to it.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"jnexus/internal/model"
)

// SourceKind one entry of the catalog
type SourceKind struct {
	Type     string `json:"type"`
	Category string `json:"category"`
	Label    string `json:"label"`
	Agent    string `json:"agent"`  // fluentbit / telegraf / otel
	Stream   string `json:"stream"` // suggested stream
	Fields   string `json:"fields"`
	Notes    string `json:"notes"`
}

// SourceCatalog every data-source type OpenObserve accepts, per its own UI grouping
var SourceCatalog = []SourceKind{
	// Web Servers
	{Type: "nginx", Category: "Web Servers", Label: "Nginx", Agent: "fluentbit", Stream: "web_nginx", Fields: "access/error logs, stub_status metrics", Notes: "tail /var/log/nginx/access.log; stub_status on 127.0.0.1:8080"},
	{Type: "apache", Category: "Web Servers", Label: "Apache HTTPD", Agent: "fluentbit", Stream: "web_apache", Fields: "access/error logs", Notes: "tail /var/log/apache2/access.log"},
	{Type: "iis", Category: "Web Servers", Label: "IIS", Agent: "fluentbit", Stream: "web_iis", Fields: "W3C logs", Notes: "Windows: Fluent Bit service, watch C:\\inetpub\\logs\\LogFiles"},
	{Type: "traefik", Category: "Web Servers", Label: "Traefik", Agent: "fluentbit", Stream: "web_traefik", Fields: "access logs (JSON)", Notes: "enable accessLog and tail the log file"},
	{Type: "caddy", Category: "Web Servers", Label: "Caddy", Agent: "fluentbit", Stream: "web_caddy", Fields: "access logs (JSON)", Notes: "tail the caddy access log"},

	// Databases (SQL pull stays in the legacy database section; these are shippers)
	{Type: "mysql_logs", Category: "Databases", Label: "MySQL (logs/metrics)", Agent: "telegraf", Stream: "db_mysql", Fields: "error log, mysql metric input", Notes: "telegraf mysql input needs a monitoring user"},
	{Type: "postgresql_logs", Category: "Databases", Label: "PostgreSQL (logs/metrics)", Agent: "telegraf", Stream: "db_postgresql", Fields: "csvlog, postgresql ext input", Notes: "enable csvlog; telegraf postgresql input"},
	{Type: "mssql_logs", Category: "Databases", Label: "SQL Server (logs)", Agent: "fluentbit", Stream: "db_mssql", Fields: "ERRORLOG tail", Notes: "tail the MSSQL ERRORLOG"},
	{Type: "oracle_alert", Category: "Databases", Label: "Oracle (alert log)", Agent: "fluentbit", Stream: "db_oracle", Fields: "alert_<SID>.log tail", Notes: "tail $ORACLE_BASE/diag/.../trace/alert log"},
	{Type: "mongo", Category: "Databases", Label: "MongoDB", Agent: "telegraf", Stream: "db_mongodb", Fields: "mongod log, mongodb input", Notes: "telegraf mongodb input"},

	// Security
	{Type: "linux_auth", Category: "Security", Label: "Linux auth.log / secure", Agent: "fluentbit", Stream: "security_linux", Fields: "sshd, sudo, su", Notes: "the platform's own Linux security stream covers this too"},
	{Type: "windows_events", Category: "Security", Label: "Windows Event Log", Agent: "fluentbit", Stream: "security_windows", Fields: "System/Application/Security", Notes: "Fluent Bit winevtlog plugin"},
	{Type: "auditd", Category: "Security", Label: "auditd", Agent: "fluentbit", Stream: "security_auditd", Fields: "/var/log/audit/audit.log", Notes: "requires auditd running"},
	{Type: "suricata", Category: "Security", Label: "Suricata IDS", Agent: "fluentbit", Stream: "security_suricata", Fields: "eve.json alerts", Notes: "tail /var/log/suricata/eve.json"},

	// DevOps
	{Type: "docker", Category: "DevOps", Label: "Docker containers", Agent: "fluentbit", Stream: "devops_docker", Fields: "container stdout/stderr", Notes: "tail /var/lib/docker/containers/*/*-json.log"},
	{Type: "kubernetes", Category: "DevOps", Label: "Kubernetes", Agent: "fluentbit", Stream: "devops_k8s", Fields: "pod logs + k8s metadata", Notes: "deploy the Fluent Bit DaemonSet with kubernetes filter"},
	{Type: "jenkins", Category: "DevOps", Label: "Jenkins", Agent: "fluentbit", Stream: "devops_jenkins", Fields: "build logs", Notes: "tail /var/log/jenkins/jenkins.log"},
	{Type: "systemd", Category: "DevOps", Label: "systemd journal", Agent: "fluentbit", Stream: "devops_journal", Fields: "journald entries", Notes: "Fluent Bit systemd input"},

	// Networking
	{Type: "syslog", Category: "Networking", Label: "Syslog (network devices)", Agent: "fluentbit", Stream: "net_syslog", Fields: "RFC3164/5424", Notes: "Fluent Bit tcp/udp syslog listener on 5140"},
	{Type: "firewall", Category: "Networking", Label: "Firewall (pfSense/iptables)", Agent: "fluentbit", Stream: "net_firewall", Fields: "firewall filter logs", Notes: "tail pf/iptables log files"},
	{Type: "haproxy", Category: "Networking", Label: "HAProxy", Agent: "fluentbit", Stream: "net_haproxy", Fields: "access/stats logs", Notes: "tail /var/log/haproxy.log"},

	// Message Queues
	{Type: "kafka", Category: "Message Queues", Label: "Kafka", Agent: "telegraf", Stream: "mq_kafka", Fields: "broker/consumer metrics", Notes: "telegraf kafka_consumer input"},
	{Type: "rabbitmq", Category: "Message Queues", Label: "RabbitMQ", Agent: "telegraf", Stream: "mq_rabbitmq", Fields: "management API metrics", Notes: "telegraf rabbitmq input (management plugin)"},
	{Type: "redis", Category: "Message Queues", Label: "Redis", Agent: "telegraf", Stream: "mq_redis", Fields: "INFO metrics, slowlog", Notes: "telegraf redis input"},

	// Languages-Frameworks
	{Type: "java_app", Category: "Languages-Frameworks", Label: "Java (log4j/logback)", Agent: "fluentbit", Stream: "app_java", Fields: "application logs", Notes: "tail the application log directory"},
	{Type: "dotnet_app", Category: "Languages-Frameworks", Label: ".NET application", Agent: "fluentbit", Stream: "app_dotnet", Fields: "Windows app logs", Notes: "tail the app log path"},
	{Type: "python_app", Category: "Languages-Frameworks", Label: "Python application", Agent: "fluentbit", Stream: "app_python", Fields: "application logs", Notes: "tail the app log path"},
	{Type: "nodejs_app", Category: "Languages-Frameworks", Label: "Node.js application", Agent: "fluentbit", Stream: "app_nodejs", Fields: "application logs", Notes: "tail the app log path or container stdout"},

	// Other
	{Type: "otel", Category: "Other", Label: "OpenTelemetry (any)", Agent: "otel", Stream: "other_otel", Fields: "logs/metrics/traces via OTLP", Notes: "deploy an OTel Collector with the OpenObserve exporter"},
	{Type: "custom_json", Category: "Other", Label: "Custom JSON (HTTP push)", Agent: "fluentbit", Stream: "other_custom", Fields: "any JSON via the external push API", Notes: "POST /api/ext/oo/{stream} or Fluent Bit http output"},
}

// FindSourceKind catalog lookup
func FindSourceKind(t string) *SourceKind {
	for i := range SourceCatalog {
		if SourceCatalog[i].Type == t {
			return &SourceCatalog[i]
		}
	}
	return nil
}

var dsiMu sync.Mutex

// ListDataSourceIntegrations returns all configured integrations
func ListDataSourceIntegrations() []model.DataSourceIntegration {
	m := SystemConfigMap()
	raw := m["oo_data_sources"]
	var out []model.DataSourceIntegration
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &out)
	}
	return out
}

func saveDataSourceIntegrations(items []model.DataSourceIntegration) error {
	b, _ := json.Marshal(items)
	return SetSystemConfigs(map[string]string{"oo_data_sources": string(b)})
}

// genCollectorCredential mints a per-integration ingest credential used by the
// generated collector configs against the platform ingest proxy endpoint.
func genCollectorCredential() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "dsrc-" + hex.EncodeToString(b), nil
}

// AddDataSourceIntegration registers a configured integration
func AddDataSourceIntegration(name, kindType, stream, hostRef, notes, operator string) (*model.DataSourceIntegration, error) {
	kind := FindSourceKind(kindType)
	if kind == nil {
		return nil, fmt.Errorf("未知数据源类型 %q", kindType)
	}
	stream = strings.TrimSpace(stream)
	if stream == "" {
		stream = kind.Stream
	}
	if !OOStreamNameValid(stream) {
		return nil, fmt.Errorf("invalid stream name")
	}
	cred, err := genCollectorCredential()
	if err != nil {
		return nil, err
	}

	dsiMu.Lock()
	defer dsiMu.Unlock()
	items := ListDataSourceIntegrations()
	if len(items) >= 200 {
		return nil, fmt.Errorf("最多保存 200 条接入")
	}
	rec := model.DataSourceIntegration{
		ID: nextIntegrationID(items), Name: strings.TrimSpace(name), Kind: kindType,
		Stream: stream, Agent: kind.Agent, HostRef: strings.TrimSpace(hostRef),
		Notes: notes, Credential: cred, Enabled: true, CreatedBy: operator, CreatedAt: time.Now(),
	}
	if rec.Name == "" {
		rec.Name = kind.Label
	}
	items = append(items, rec)
	if err := saveDataSourceIntegrations(items); err != nil {
		return nil, err
	}
	return &rec, nil
}

func nextIntegrationID(items []model.DataSourceIntegration) int64 {
	var max int64
	for _, it := range items {
		if it.ID > max {
			max = it.ID
		}
	}
	return max + 1
}

// DeleteDataSourceIntegration removes one integration by id
func DeleteDataSourceIntegration(id int64) error {
	dsiMu.Lock()
	defer dsiMu.Unlock()
	items := ListDataSourceIntegrations()
	out := items[:0]
	found := false
	for _, it := range items {
		if it.ID == id {
			found = true
			continue
		}
		out = append(out, it)
	}
	if !found {
		return fmt.Errorf("接入不存在")
	}
	return saveDataSourceIntegrations(out)
}

// SetDataSourceIntegrationEnabled flips the enabled flag
func SetDataSourceIntegrationEnabled(id int64, enabled bool) error {
	dsiMu.Lock()
	defer dsiMu.Unlock()
	items := ListDataSourceIntegrations()
	for i := range items {
		if items[i].ID == id {
			items[i].Enabled = enabled
			return saveDataSourceIntegrations(items)
		}
	}
	return fmt.Errorf("接入不存在")
}

// CollectorConfig renders a ready-to-paste collector config for an integration
func CollectorConfig(kindType, stream, credential string) (string, error) {
	kind := FindSourceKind(kindType)
	if kind == nil {
		return "", fmt.Errorf("未知数据源类型 %q", kindType)
	}
	if stream == "" {
		stream = kind.Stream
	}
	base := SystemConfigMap()
	ooURL := strings.TrimSuffix(base["oo_url"], "/")
	if ooURL == "" {
		return "", fmt.Errorf("请先在 系统设置 -> 可观测集成 中配置 OpenObserve 地址")
	}
	org := base["oo_org"]
	if org == "" {
		org = "default"
	}
	ingest := fmt.Sprintf("%s/api/%s/%s/_json", ooURL, org, stream)

	switch kind.Agent {
	case "fluentbit":
		return fluentBitConfig(*kind, ingest, credential), nil
	case "telegraf":
		return telegrafConfig(kind, ingest, credential), nil
	case "otel":
		return otelConfig(ingest, credential), nil
	}
	return "", fmt.Errorf("unsupported agent %q", kind.Agent)
}

func inputHint(kind SourceKind) string {
	switch kind.Type {
	case "nginx":
		return "/var/log/nginx/access.log"
	case "apache":
		return "/var/log/apache2/access.log"
	case "iis":
		return "C:\\inetpub\\logs\\LogFiles\\W3SVC1\\*.log"
	case "linux_auth":
		return "/var/log/auth.log"
	case "auditd":
		return "/var/log/audit/audit.log"
	case "suricata":
		return "/var/log/suricata/eve.json"
	case "docker":
		return "/var/lib/docker/containers/*/*-json.log"
	case "jenkins":
		return "/var/log/jenkins/jenkins.log"
	case "mssql_logs":
		return "C:\\Program Files\\Microsoft SQL Server\\MSSQL*.MSSQLSERVER\\MSSQL\\Log\\ERRORLOG"
	case "oracle_alert":
		return "/opt/oracle/diag/rdbms/*/*/trace/alert_*.log"
	}
	return "/var/log/<path-to-log>"
}

func splitIngest(ingest string) (host, port, uri string) {
	s := ingest
	for _, pfx := range []string{"https://", "http://"} {
		s = strings.TrimPrefix(s, pfx)
	}
	rest := s
	if i := strings.Index(s, "/"); i >= 0 {
		rest = s[:i]
		uri = s[i:]
	}
	host = rest
	port = "80"
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		host = rest[:i]
		port = rest[i+1:]
	}
	if uri == "" {
		uri = "/"
	}
	return
}

func fluentBitConfig(kind SourceKind, ingest, credential string) string {
	host, port, uri := splitIngest(ingest)
	input := `[INPUT]
    Name   tail
    Path   ` + inputHint(kind) + `
    Tag    ` + kind.Stream + `
    Refresh_Interval 5`
	return `# JNexus -> OpenObserve: ` + kind.Label + ` (` + kind.Category + `)
# 1) adjust Path to the real environment  2) start Fluent Bit  3) data lands in stream ` + kind.Stream + `
[SERVICE]
    Flush        5
    Daemon       Off
    Log_Level    info

` + input + `

[FILTER]
    Name         record_modifier
    Match        ` + kind.Stream + `
    Record       datasource ` + kind.Type + `

[OUTPUT]
    Name         http
    Match        ` + kind.Stream + `
    Host         ` + host + `
    Port         ` + port + `
    Uri          ` + uri + `
    tls          On
    tls.verify   Off
    format       json
    json_date_key _time
    json_date_format iso8601
    HTTP_User    ` + credential + `
    HTTP_Passwd  x
    Retry_Limit  5

# ` + kind.Notes
}

func telegrafConfig(kind *SourceKind, ingest, credential string) string {
	return `# JNexus -> OpenObserve: ` + kind.Label + ` (` + kind.Category + `)
# 1) install telegraf  2) replace <input-plugin> below with the real plugin  3) start

[agent]
    interval = "10s"
    flush_interval = "10s"

[[inputs.<input-plugin>]]  # ` + kind.Notes + `

[[outputs.http]]
    url = "` + ingest + `"
    data_format = "json"
    [outputs.http.headers]
       Authorization = "` + credential + `"
       Content-Type = "application/json"
`
}

func otelConfig(ingest, credential string) string {
	return `# JNexus -> OpenObserve: OpenTelemetry Collector
# any OTLP data (logs/metrics/traces) lands in OpenObserve
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:

exporters:
  otlphttp/openobserve:
    endpoint: "` + ingest + `"
    headers:
      Authorization: "` + credential + `"

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/openobserve]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/openobserve]
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/openobserve]
`
}
