// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"fmt"
	"sync"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/sshpool"
)

// 预设报告模板（命令为 POSIX sh，采集失败段落不影响其余输出）
type ReportTemplate struct {
	Key  string
	Name string
	Desc string
	Cmd  string
}

var reportTemplates = []ReportTemplate{
	{
		Key:  "accounts",
		Name: "Server Accounts",
		Desc: "All system accounts (UID/Shell/Home) and login-enabled accounts",
		Cmd: `echo '== Summary =='
TOTAL=$(awk -F: 'END{print NR}' /etc/passwd 2>/dev/null)
LOGIN=$(awk -F: '$7 !~ /nologin|false/ && $7 != ""' /etc/passwd 2>/dev/null | wc -l)
HUMAN=$(awk -F: '$3 >= 1000 && $1 != "nobody"' /etc/passwd 2>/dev/null | wc -l)
UID0=$(awk -F: '$3 == 0 {printf "%s ",$1}' /etc/passwd 2>/dev/null)
DUP=$(awk -F: '{print $3}' /etc/passwd 2>/dev/null | sort -n | uniq -d | tr '\n' ' ')
echo "total_accounts=${TOTAL:-0}"
echo "login_enabled=${LOGIN:-0}"
echo "human_accounts=${HUMAN:-0}"
echo "system_accounts=$((TOTAL - HUMAN))"
echo "uid0_accounts=${UID0:-none}"
echo "duplicate_uid=${DUP:-none}"
echo
echo '== Accounts CSV =='
echo 'username,uid,gid,group,home,shell,login_enabled,type'
awk -F: 'NR==FNR { g[$3]=$1; next } { le = ($7 ~ /nologin|false/) ? "no" : "yes"; ty = ($3 >= 1000 && $1 != "nobody") ? "human" : "system"; printf "%s,%s,%s,%s,%s,%s,%s,%s\n", $1,$3,$4,g[$4],$6,$7,le,ty }' /etc/group /etc/passwd 2>/dev/null
echo
echo '== Login-enabled Detail =='
grep -Ev '(nologin|false)$' /etc/passwd 2>/dev/null | cut -d: -f1,6,7
`,
	},
	{
		Key:  "crontab",
		Name: "Crontab Jobs",
		Desc: "All user crontabs and /etc/cron.d listings",
		Cmd: `echo '== User crontabs =='
for u in $(cut -d: -f1 /etc/passwd 2>/dev/null); do c=$(crontab -l -u "$u" 2>/dev/null); [ -n "$c" ] && echo "--- $u ---" && echo "$c"; done
echo '== /etc/cron.d =='
ls -l /etc/cron.d 2>/dev/null || echo 'N/A'
`,
	},
	{
		Key:  "health",
		Name: "Health Check",
		Desc: "Uptime/load, memory, disk usage, top CPU processes",
		Cmd: `echo '== Host/Uptime/Load =='
hostname
uptime 2>/dev/null || cat /proc/uptime
echo '== Memory =='
free -m 2>/dev/null || head -3 /proc/meminfo
echo '== Disk Usage =='
df -hP 2>/dev/null | grep -v tmpfs || true
echo '== CPU TOP5 =='
ps aux 2>/dev/null | sort -rk3 | head -6 || true
`,
	},
	{
		Key:  "osinfo",
		Name: "System Info",
		Desc: "Kernel version, distribution, IP addresses",
		Cmd: `echo '== Kernel =='
uname -a
echo '== Distribution =='
head -3 /etc/os-release 2>/dev/null || echo 'N/A'
echo '== IP Addresses =='
(ip -4 addr 2>/dev/null || ifconfig 2>/dev/null || hostname -I 2>/dev/null) | grep -w inet 2>/dev/null || hostname
`,
	},
	{
		Key:  "portcert",
		Name: "Ports & Certificates",
		Desc: "Listening ports, HTTP/HTTPS detection, HTTPS certificate and local certificate file expiry check",
		Cmd: `echo '== Listening Ports =='
LISTEN=$( (ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null) | grep -i listen )
printf '%s\n' "$LISTEN" | head -100
[ -z "$LISTEN" ] && echo 'N/A'
echo
echo '== Port Protocol Detection (http/https) =='
TO=""
command -v timeout >/dev/null 2>&1 && TO="timeout 3"
ADDRS=$( (ss -tln 2>/dev/null | awk 'NR>1 {print $4}') ; (netstat -tln 2>/dev/null | awk 'NR>2 {print $4}') )
PLIST=$(for a in $ADDRS; do p=${a##*:}; case "$p" in ''|*[!0-9]*) continue ;; esac; echo "$p"; done | sort -n | uniq | head -64)
n=0
for p in $PLIST; do
  n=$((n+1))
  proto=other; info=""
  if command -v openssl >/dev/null 2>&1; then
    if echo | $TO openssl s_client -connect 127.0.0.1:$p 2>/dev/null | grep -q 'BEGIN CERTIFICATE'; then
      proto=https
      cert=$(echo | $TO openssl s_client -connect 127.0.0.1:$p 2>/dev/null | openssl x509 -noout -subject -enddate 2>/dev/null | tr '\n' ' ')
      [ -n "$cert" ] && info="$cert"
    fi
  fi
  if [ "$proto" = other ] && command -v bash >/dev/null 2>&1; then
    line=$($TO bash -c "exec 3<>/dev/tcp/127.0.0.1/$p && printf 'HEAD / HTTP/1.0\r\n\r\n' >&3 && head -c 16 <&3" 2>/dev/null)
    case "$line" in HTTP/*) proto=http ;; esac
  fi
  echo "port $p: $proto $info"
done
[ "$n" = 0 ] && echo 'No listening ports detected.'
echo
echo '== Local Certificate Expiry Check =='
NOW=$(date +%s)
FILES=$( (find /etc/ssl/certs /etc/pki/tls/certs /etc/grid-security /etc/kubernetes/ssl -maxdepth 3 -type f 2>/dev/null; find /etc/ssl /etc/pki -maxdepth 5 -type f \( -name '*.pem' -o -name '*.crt' -o -name '*.cer' \) 2>/dev/null) | sort -u | head -300 )
if [ -z "$FILES" ]; then
  echo 'No certificate directories/files found (/etc/ssl, /etc/pki).'
else
  exp=0; ok=0
  for f in $FILES; do
    [ -f "$f" ] || continue
    END=$(openssl x509 -enddate -noout -in "$f" 2>/dev/null | cut -d= -f2)
    [ -z "$END" ] && continue
    ES=$(date -d "$END" +%s 2>/dev/null || date -j -f '%b %d %H:%M:%S %Y %Z' "$END" +%s 2>/dev/null)
    if [ -z "$ES" ]; then
      echo "UNKNOWN  $END  $f"
    elif [ "$ES" -lt "$NOW" ]; then
      echo "EXPIRED  $END  $f"
      exp=$((exp+1))
    else
      ok=$((ok+1))
    fi
  done
  echo "-- Certificate files: $ok valid, $exp expired"
fi
`,
	},
}

// GetReportTemplates 供前端展示模板列表
func ReportTemplateList() []ReportTemplate { return reportTemplates }

// FindReportTemplate 导出查询（handler 展示模板名用）
func FindReportTemplate(key string) (ReportTemplate, error) {
	return findTemplate(key)
}

func findTemplate(key string) (ReportTemplate, error) {
	for _, t := range reportTemplates {
		if t.Key == key {
			return t, nil
		}
	}
	return ReportTemplate{}, fmt.Errorf("未知报告模板: %s", key)
}

// StartReport 创建报告并并发采集（复用主机默认凭据连接）
func StartReport(operator *model.User, templateKey string, hostIDs []uint) (uint, error) {
	tpl, err := findTemplate(templateKey)
	if err != nil {
		return 0, err
	}
	var hosts []model.Host
	if len(hostIDs) > 0 {
		if err := model.DB.Where("id IN ?", hostIDs).Find(&hosts).Error; err != nil {
			return 0, err
		}
	} else {
		if err := model.DB.Find(&hosts).Error; err != nil {
			return 0, err
		}
	}
	if len(hosts) == 0 {
		return 0, fmt.Errorf("no target hosts selected")
	}

	now := time.Now()
	report := model.Report{
		Name:      fmt.Sprintf("%s %s", tpl.Name, now.Format("20060102-150405")),
		Template:  tpl.Key,
		Operator:  operator.Username,
		HostCount: len(hosts),
		Status:    "running",
		CreatedAt: now,
	}
	if err := model.DB.Create(&report).Error; err != nil {
		return 0, err
	}
	for _, h := range hosts {
		model.DB.Create(&model.ReportItem{
			ReportID: report.ID, HostID: h.ID,
			HostName: h.Name, HostIP: h.IP, Status: "pending",
			CreatedAt: now,
		})
	}

	go runReport(tpl, report.ID, hosts)
	return report.ID, nil
}

func runReport(tpl ReportTemplate, reportID uint, hosts []model.Host) {
	defer func() { recover() }()
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	for i := range hosts {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			collectHostReport(tpl, reportID, hosts[idx])
		}(i)
	}
	wg.Wait()
	finishReport(reportID)
}

func collectHostReport(tpl ReportTemplate, reportID uint, host model.Host) {
	var item model.ReportItem
	if err := model.DB.Where("report_id = ? AND host_id = ?", reportID, host.ID).First(&item).Error; err != nil {
		return
	}
	model.DB.Model(&item).Update("status", "running")

	cli, err := sshpool.ClientFor(&host)
	if err != nil {
		model.DB.Model(&item).Updates(map[string]any{
			"status": "failed", "error": err.Error(),
		})
		return
	}
	defer cli.Close()

	out, code, rerr := runCapture(cli, tpl.Cmd)
	updates := map[string]any{
		"content": out, "created_at": time.Now(),
	}
	if rerr != nil {
		updates["status"] = "failed"
		updates["error"] = rerr.Error()
	} else if code != 0 {
		updates["status"] = "failed"
		updates["error"] = fmt.Sprintf("采集命令退出码 %d", code)
	} else {
		updates["status"] = "success"
	}
	model.DB.Model(&item).Updates(updates)
}

func finishReport(reportID uint) {
	now := time.Now()
	model.DB.Model(&model.Report{}).Where("id = ?", reportID).
		Updates(map[string]any{"status": "done", "finished_at": now})
}
