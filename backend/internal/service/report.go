// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"fmt"
	"sync"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/sshpool"
)

// Preset report templates (commands are POSIX sh; a failing section does not affect the rest of the output).
// Dual-version like the script center: Cmd runs on Linux via SSH, CmdPs on Windows via WinRM.
type ReportTemplate struct {
	Key   string
	Name  string
	Desc  string
	Cmd   string
	CmdPs string
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
		CmdPs: `Write-Output '== Summary =='
$u = @(Get-LocalUser 2>$null | Select-Object Name,Enabled,Description,LastLogon)
if ($u.Count -eq 0) { $u = @(Get-CimInstance Win32_UserAccount 2>$null | Select-Object Name,Enabled,Description) }
$enabled = @($u | Where-Object { $_.Enabled })
Write-Output ("total_accounts=" + $u.Count)
Write-Output ("login_enabled=" + $enabled.Count)
Write-Output ("disabled_accounts=" + ($u.Count - $enabled.Count))
Write-Output ''
Write-Output '== Accounts CSV =='
Write-Output 'username,enabled,description,last_logon,type'
$u | ForEach-Object {
  $ll = if ($_.LastLogon) { $_.LastLogon.ToString('yyyy-MM-dd HH:mm:ss') } else { 'never' }
  $ty = if ($_.Name -match '^(Administrator|Guest|DefaultAccount|WDAGUtilityAccount)$') { 'system' } else { 'human' }
  $de = ($_.Description -replace '[,;\r\n]', ' ')
  Write-Output ("{0},{1},{2},{3},{4}" -f $_.Name, $(if ($_.Enabled) { 'yes' } else { 'no' }), $de, $ll, $ty)
}
Write-Output ''
Write-Output '== Enabled Detail =='
$enabled | ForEach-Object { Write-Output ("{0}  {1}" -f $_.Name, $_.Description) }
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
		CmdPs: `Write-Output '== Scheduled Tasks (non-Microsoft) =='
$t = @(Get-ScheduledTask 2>$null | Where-Object { $_.TaskPath -notlike '\Microsoft\*' } | Select-Object -First 80)
$t | ForEach-Object {
  $info = $_ | Get-ScheduledTaskInfo 2>$null
  $act = $_.Actions | Select-Object -First 1
  Write-Output ("path={0} name={1} state={2} last={3} next={4} action={5} {6}" -f $_.TaskPath, $_.TaskName, $_.State, $info.LastRunTime, $info.NextRunTime, $act.Execute, $act.Arguments)
}
if ($t.Count -eq 0) { Write-Output 'N/A' }
Write-Output ''
Write-Output '== Count =='
$all = @(Get-ScheduledTask 2>$null)
Write-Output ("total_tasks=" + $all.Count + " custom_tasks=" + @(Get-ScheduledTask 2>$null | Where-Object { $_.TaskPath -notlike '\Microsoft\*' }).Count)
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
		CmdPs: `Write-Output '== Host/Uptime =='
hostname
$os = Get-CimInstance Win32_OperatingSystem
$up = (Get-Date) - $os.LastBootUpTime
Write-Output ("up_days={0} up_hours={1}" -f [int]$up.TotalDays, $up.Hours)
Write-Output '== Memory (MB) =='
Write-Output ("total_mb={0} free_mb={1} used_pct={2}" -f [int]($os.TotalVisibleMemorySize/1KB), [int]($os.FreePhysicalMemory/1KB), [math]::Round(100*(1-$os.FreePhysicalMemory/$os.TotalVisibleMemorySize),1))
Write-Output '== Disk Usage =='
Get-CimInstance Win32_LogicalDisk -Filter 'DriveType=3' | ForEach-Object {
  $pct = if ($_.Size) { [string][int](100*(1-$_.FreeSpace/$_.Size)) + '%' } else { 'N/A' }
  Write-Output ("{0} {1}GB total {2}GB free {3} used" -f $_.DeviceID, [int]($_.Size/1GB), [int]($_.FreeSpace/1GB), $pct)
}
Write-Output '== CPU TOP5 =='
Get-Process | Sort-Object CPU -Descending | Select-Object -First 5 | ForEach-Object {
  Write-Output ("{0} cpu_s={1} mem_mb={2}" -f $_.Name, [int]$_.CPU, [int]($_.WorkingSet64/1MB))
}
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
		CmdPs: `Write-Output '== System =='
$cs = Get-CimInstance Win32_ComputerSystem
$os = Get-CimInstance Win32_OperatingSystem
Write-Output ("hostname=" + $env:COMPUTERNAME)
Write-Output ("os=" + $os.Caption + " " + $os.Version)
Write-Output ("domain=" + $cs.Domain)
Write-Output ("last_boot=" + $os.LastBootUpTime)
Write-Output '== IP Addresses =='
Get-NetIPAddress -AddressFamily IPv4 2>$null | Where-Object { $_.IPAddress -ne '127.0.0.1' } | ForEach-Object {
  Write-Output ("inet {0} if={1}" -f $_.IPAddress, $_.InterfaceAlias)
}
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
		CmdPs: `Write-Output '== Listening Ports =='
$ports = @(Get-NetTCPConnection -State Listen 2>$null | Select-Object -ExpandProperty LocalPort | Where-Object { $_ -gt 0 } | Sort-Object -Unique | Select-Object -First 100)
if ($ports.Count -eq 0) { Write-Output 'N/A' } else { $ports | ForEach-Object { Write-Output ("port {0}: tcp listen" -f $_) } }
Write-Output ''
Write-Output '== Port Protocol Detection (http/https) =='
$crl = New-Object System.Net.Security.RemoteCertificateValidationCallback({ $true })
foreach ($p in ($ports | Select-Object -First 64)) {
  $proto = 'other'; $info = ''
  $tcp = New-Object Net.Sockets.TcpClient
  try {
    $tcp.Connect('127.0.0.1', $p)
    $tcp.ReceiveTimeout = 3000; $tcp.SendTimeout = 3000
    $ns = $tcp.GetStream(); $ns.ReadTimeout = 3000; $ns.WriteTimeout = 3000
    $ssl = New-Object Net.Security.SslStream($ns, $false, $crl)
    try {
      $ssl.AuthenticateAsClient('127.0.0.1')
      $c = New-Object Security.Cryptography.X509Certificates.X509Certificate2($ssl.RemoteCertificate)
      $proto = 'https'
      $days = [int]($c.NotAfter - (Get-Date)).TotalDays
      $info = 'subject=' + $c.Subject + ' notAfter=' + $c.NotAfter.ToString('yyyy-MM-dd') + ' (' + $days + 'd)'
    } finally { $ssl.Dispose() }
  } catch {
    try {
      $tcp3 = New-Object Net.Sockets.TcpClient
      $tcp3.ReceiveTimeout = 3000; $tcp3.SendTimeout = 3000
      $tcp3.Connect('127.0.0.1', $p)
      $s = $tcp3.GetStream(); $s.ReadTimeout = 3000; $s.WriteTimeout = 3000
        $req = [Text.Encoding]::ASCII.GetBytes('HEAD / HTTP/1.0' + [char]13 + [char]10 + [char]13 + [char]10)
        $s.Write($req, 0, $req.Length)
        $buf = New-Object byte[] 16
        $n = $s.Read($buf, 0, 16)
        if ($n -gt 0 -and [Text.Encoding]::ASCII.GetString($buf, 0, $n).StartsWith('HTTP/')) { $proto = 'http' }
        $tcp3.Close()
      } catch { }
  }
  try { $tcp.Close() } catch { }
  Write-Output ("port {0}: {1} {2}" -f $p, $proto, $info)
}
Write-Output ''
Write-Output '== Local Certificate Expiry Check (LocalMachine\My) =='
$now = Get-Date
$certs = @(Get-ChildItem Cert:\LocalMachine\My -ErrorAction SilentlyContinue)
$exp = 0; $ok = 0
foreach ($c in $certs) {
  if ($c.NotAfter -lt $now) {
    Write-Output ("EXPIRED  {0}  {1}" -f $c.NotAfter.ToString('yyyy-MM-dd'), $c.Subject)
    $exp++
  } else { $ok++ }
}
Write-Output ("-- Certificate files: {0} valid, {1} expired" -f $ok, $exp)
`,
	},
}

// GetReportTemplates serves the template list for the frontend
func ReportTemplateList() []ReportTemplate { return reportTemplates }

// FindReportTemplate exported lookup (handlers use it to show template names)
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

// StartReport creates a report and collects concurrently (reuses the host default credential connection)
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

	if IsWindows(&host) {
		collectHostReportWin(tpl, item, host)
		return
	}

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

// collectHostReportWin collects preset reports on Windows hosts: the PowerShell
// template variant runs via WinRM with the host default credential (dual-version
// dispatch mirrors the script center). Output line formats stay parser-compatible
// with the ports & certificates matrix.
func collectHostReportWin(tpl ReportTemplate, item model.ReportItem, host model.Host) {
	if tpl.CmdPs == "" {
		model.DB.Model(&item).Updates(map[string]any{
			"status": "failed", "error": "该报告模板未提供 Windows (PowerShell) 版本", "created_at": time.Now(),
		})
		return
	}
	var user, pass string
	cred := defaultCredential(&host)
	if cred != nil {
		user = cred.Username
		p, err := pkgDec(cred.Password)
		if err != nil {
			model.DB.Model(&item).Updates(map[string]any{"status": "failed", "error": "凭据解密失败: " + err.Error(), "created_at": time.Now()})
			return
		}
		pass = p
	} else if host.Password != "" {
		p, err := pkgDec(host.Password)
		if err != nil {
			model.DB.Model(&item).Updates(map[string]any{"status": "failed", "error": "凭据解密失败: " + err.Error(), "created_at": time.Now()})
			return
		}
		pass = p
	} else {
		model.DB.Model(&item).Updates(map[string]any{"status": "failed", "error": "Windows 主机无可用账号凭据", "created_at": time.Now()})
		return
	}
	timeout := 120
	if tpl.Key == "portcert" {
		timeout = 300
	}
	out, code, err := WinRMRun(&host, user, pass, tpl.CmdPs, timeout)
	updates := map[string]any{"content": out, "created_at": time.Now()}
	if err != nil {
		updates["status"] = "failed"
		updates["error"] = err.Error()
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
