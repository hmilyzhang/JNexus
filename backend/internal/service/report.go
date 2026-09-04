// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"fmt"
	"sync"
	"time"

	"autoops/internal/model"
	"autoops/internal/sshpool"
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
		Name: "服务器账号信息",
		Desc: "系统全部账号（UID/Shell/Home）、可登录账号列表",
		Cmd: `echo '== 系统账号 =='
getent passwd 2>/dev/null || cat /etc/passwd
echo
echo '== 可登录账号 =='
grep -Ev '(nologin|false)$' /etc/passwd 2>/dev/null | cut -d: -f1,6,7
`,
	},
	{
		Key:  "crontab",
		Name: "Crontab 定时任务",
		Desc: "全部用户 crontab 与 /etc/cron.d 列表",
		Cmd: `echo '== 用户 crontab =='
for u in $(cut -d: -f1 /etc/passwd 2>/dev/null); do c=$(crontab -l -u "$u" 2>/dev/null); [ -n "$c" ] && echo "--- $u ---" && echo "$c"; done
echo '== /etc/cron.d =='
ls -l /etc/cron.d 2>/dev/null || echo 'N/A'
`,
	},
	{
		Key:  "health",
		Name: "健康检查",
		Desc: "运行时间/负载、内存、磁盘使用率、CPU TOP 进程",
		Cmd: `echo '== 主机/运行时间/负载 =='
hostname
uptime 2>/dev/null || cat /proc/uptime
echo '== 内存 =='
free -m 2>/dev/null || head -3 /proc/meminfo
echo '== 磁盘使用 =='
df -hP 2>/dev/null | grep -v tmpfs || true
echo '== CPU 占用 TOP5 =='
ps aux 2>/dev/null | sort -rk3 | head -6 || true
`,
	},
	{
		Key:  "osinfo",
		Name: "系统信息",
		Desc: "内核版本、发行版、IP 地址",
		Cmd: `echo '== 内核 =='
uname -a
echo '== 发行版 =='
head -3 /etc/os-release 2>/dev/null || echo 'N/A'
echo '== IP 地址 =='
(ip -4 addr 2>/dev/null || ifconfig 2>/dev/null || hostname -I 2>/dev/null) | grep -w inet 2>/dev/null || hostname
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
		return 0, fmt.Errorf("未选择任何目标主机")
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
