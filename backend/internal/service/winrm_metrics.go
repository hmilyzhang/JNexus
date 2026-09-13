// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"strconv"
	"strings"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
)

// Windows host metric collection: PowerShell CIM instances (pinned InvariantCulture output).
// Collection frequency is the same as Linux (monitor_interval_sec); WinRM is more expensive and shares throttling with execution.

const winMetricCmd = `$ErrorActionPreference='SilentlyContinue'
$r = [System.Globalization.CultureInfo]::InvariantCulture
$cpu = (Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average
$os = Get-CimInstance Win32_OperatingSystem
$memTotal = [math]::Round($os.TotalVisibleMemorySize/1KB,1)
$memFree  = [math]::Round($os.FreePhysicalMemory/1KB,1)
$memPct = if ($memTotal -gt 0) { [math]::Round((($memTotal-$memFree)/$memTotal)*100,1) } else { 0 }
$boot = $os.LastBootUpTime.ToString('yyyy-MM-ddTHH:mm:ss')
Write-Output ('---WINMETRIC---')
Write-Output ('CPU=' + $cpu)
Write-Output ('MEM=' + $memPct)
Write-Output ('BOOT=' + $boot)
Get-CimInstance Win32_LogicalDisk -Filter 'DriveType=3' | ForEach-Object {
  $pct = if ($_.Size -gt 0) { [math]::Round(($_.Size-$_.FreeSpace)/$_.Size*100,1) } else { 0 }
  Write-Output ('DISK=' + $_.DeviceID + ' ' + $pct)
}`

// collectWindowsMetrics collects Windows host CPU/memory/disk via WinRM and records a sample
func collectWindowsMetrics(h *model.Host) {
	defer func() { recover() }()
	var user, pass string
	cred := defaultCredential(h)
	if cred != nil {
		user = cred.Username
		p, err := pkgDec(cred.Password)
		if err != nil {
			return
		}
		pass = p
	} else if h.Password != "" {
		p, err := pkgDec(h.Password)
		if err != nil {
			return
		}
		pass = p
	} else {
		return // no credential, WinRM impossible
	}
	out, _, err := WinRMRun(h, user, pass, winMetricCmd, 45)
	if err != nil || out == "" {
		return
	}
	s, ok := parseWinMetricOutput(out)
	if !ok {
		return
	}
	model.DB.Create(&model.HostMetric{
		HostID: h.ID, CPUPercent: s.CPU, MemPercent: s.Mem, DiskPercent: s.Disk,
		CollectedAt: time.Now(),
	})
	// Windows reboot detection: push when the boot time changes
	if h.LastBootID != "" && s.BootID != "" && s.BootID != h.LastBootID {
		SendHostRebootAlert(h, s.BootID)
		LogAlertEvent("host_reboot", "warn", h.Name, "Windows 主机系统重启（启动时间变化）")
	}
	if s.BootID != "" && s.BootID != h.LastBootID {
		model.DB.Model(&model.Host{}).Where("id = ?", h.ID).Update("last_boot_id", s.BootID)
	}
}

// parseWinMetricOutput parses the ---WINMETRIC--- output
func parseWinMetricOutput(out string) (hostSample, bool) {
	var s hostSample
	idx := strings.Index(out, "---WINMETRIC---")
	if idx < 0 {
		return s, false
	}
	body := out[idx+len("---WINMETRIC---"):]
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "CPU="):
			s.CPU, _ = strconv.ParseFloat(strings.TrimPrefix(line, "CPU="), 64)
		case strings.HasPrefix(line, "MEM="):
			s.Mem, _ = strconv.ParseFloat(strings.TrimPrefix(line, "MEM="), 64)
		case strings.HasPrefix(line, "BOOT="):
			s.BootID = strings.TrimPrefix(line, "BOOT=")
		case strings.HasPrefix(line, "DISK="):
			// take the highest usage among all logical drives (consistent with Linux behavior)
			parts := strings.SplitN(strings.TrimPrefix(line, "DISK="), " ", 2)
			if len(parts) == 2 {
				if p, e := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); e == nil && p > s.Disk {
					s.Disk = p
				}
			}
		}
	}
	if s.CPU == 0 && s.Mem == 0 {
		return s, false
	}
	return s, true
}

// pkgDec decrypts a credential ciphertext
func pkgDec(cipherB64 string) (string, error) {
	return pkg.Decrypt(cipherB64)
}
