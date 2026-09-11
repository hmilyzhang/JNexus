// JNexus 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/service"
)

// ListReportTemplates 预设报告模板列表
func ListReportTemplates(c *gin.Context) {
	out := make([]gin.H, 0, len(service.ReportTemplateList()))
	for _, t := range service.ReportTemplateList() {
		out = append(out, gin.H{"key": t.Key, "name": t.Name, "desc": t.Desc})
	}
	c.JSON(http.StatusOK, out)
}

type reportReq struct {
	Template string `json:"template" binding:"required"`
	HostIDs  []uint `json:"host_ids"`
}

// CreateReport 生成报告（按模板在目标主机采集，异步执行）
func CreateReport(c *gin.Context) {
	var req reportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request (template required)"})
		return
	}
	reportID, err := service.StartReport(currentUser(c), req.Template, req.HostIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"report_id": reportID})
}

func findReport(c *gin.Context) (*model.Report, []model.ReportItem, bool) {
	id, _ := strconv.Atoi(c.Param("id"))
	var report model.Report
	if err := model.DB.First(&report, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		return nil, nil, false
	}
	var items []model.ReportItem
	model.DB.Where("report_id = ?", id).Order("status DESC, host_name").Find(&items)
	return &report, items, true
}

// GetReport 报告详情（含逐主机采集结果，失败的排前）
func GetReport(c *gin.Context) {
	report, items, ok := findReport(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"report": report, "items": items})
}

// ListReports 报告列表
func ListReports(c *gin.Context) {
	var reports []model.Report
	q := model.DB
	if tpl := c.Query("template"); tpl != "" {
		q = q.Where("template = ?", tpl)
	}
	q.Order("id DESC").Limit(100).Find(&reports)
	c.JSON(http.StatusOK, reports)
}

// DeleteReport 删除报告（仅系统管理员）
func DeleteReport(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	model.DB.Where("report_id = ?", id).Delete(&model.ReportItem{})
	model.DB.Delete(&model.Report{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ExportReport 报告导出：?format=log（汇总文本）| csv（主机状态表）
func ExportReport(c *gin.Context) {
	report, items, ok := findReport(c)
	if !ok {
		return
	}
	stamp := time.Now().Format("20060102_150405")
	format := c.DefaultQuery("format", "log")

	if format == "csv" {
		var sb strings.Builder
		sb.WriteString("\uFEFF")
		sb.WriteString("Hostname,IP,Status,Error,Content\n")
		for _, it := range items {
			err := it.Error
			row := []string{it.HostName, it.HostIP, it.Status, err, it.Content}
			for i := range row {
				row[i] = strings.ReplaceAll(row[i], "\"", "\"\"")
				if strings.ContainsAny(row[i], ",\n\r") {
					row[i] = "\"" + row[i] + "\""
				}
			}
			sb.WriteString(strings.Join(row, ",") + "\n")
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=report-%d-%s.csv", int(report.ID), stamp))
		c.Data(http.StatusOK, "application/octet-stream", []byte(sb.String()))
		return
	}

	tpl, err := service.FindReportTemplate(report.Template)
	tplName := report.Template
	if err == nil {
		tplName = tpl.Name
	}
	var sb strings.Builder
	sb.WriteString("================ JNexus Collection Report #" + strconv.Itoa(int(report.ID)) + " ================\n")
	sb.WriteString("Template: " + tplName + "    Operator: " + report.Operator + "    Time: " + report.CreatedAt.Format("2006-01-02 15:04:05") + "\n")
	sb.WriteString(fmt.Sprintf("Targets: %d hosts\n\n", len(items)))
	for _, it := range items {
		st := "Success"
		if it.Status == "failed" {
			st = "Failed"
		}
		sb.WriteString(fmt.Sprintf("---------- [%s] %s (%s) ----------\n", st, it.HostName, it.HostIP))
		if it.Error != "" {
			sb.WriteString("Error: " + it.Error + "\n")
		}
		if strings.TrimSpace(it.Content) == "" {
			sb.WriteString("(no output)\n")
		} else {
			sb.WriteString(it.Content)
			if !strings.HasSuffix(it.Content, "\n") {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("================ Exported at " + time.Now().Format("2006-01-02 15:04:05") + " ================\n")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=report-%d-%s.log", int(report.ID), stamp))
	c.Data(http.StatusOK, "application/octet-stream", []byte(sb.String()))
}

// ---- 跨主机账号对比（accounts 模板报告的 CSV 解析聚合） ----

type acctCSVRow struct {
	Username     string
	UID          int
	Group        string
	Shell        string
	LoginEnabled bool
	Type         string // human / system
}

// parseAccountsCSV 从报告条目内容中解析 == Accounts CSV == 段（v1.180+ 新格式）
func parseAccountsCSV(content string) []acctCSVRow {
	inCSV := false
	rows := []acctCSVRow{}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "== Accounts CSV ==") {
			inCSV = true
			continue
		}
		if !inCSV {
			continue
		}
		if line == "" || strings.HasPrefix(line, "==") {
			break
		}
		if strings.HasPrefix(line, "username,") {
			continue // 表头
		}
		p := strings.Split(line, ",")
		if len(p) < 8 {
			continue
		}
		uid, err := strconv.Atoi(p[1])
		if err != nil {
			continue
		}
		rows = append(rows, acctCSVRow{
			Username: p[0], UID: uid, Group: p[3], Shell: p[5],
			LoginEnabled: p[6] == "yes", Type: p[7],
		})
	}
	return rows
}

// ReportAccountsMatrix GET /api/reports/accounts-matrix/:id
// 输出：账号 × 主机 矩阵（present 为主机列下标）+ 可疑账号清单（UID0 非 root / 系统账号可登录 / UID 重复）
func ReportAccountsMatrix(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var report model.Report
	if err := model.DB.First(&report, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "report not found"})
		return
	}
	var items []model.ReportItem
	model.DB.Where("report_id = ?", id).Order("host_name").Find(&items)

	type hostCol struct {
		ItemID   uint   `json:"item_id"`
		HostName string `json:"host_name"`
		HostIP   string `json:"host_ip"`
	}
	hostsOut := []hostCol{}

	type acctAgg struct {
		Username     string `json:"username"`
		UID          int    `json:"uid"`
		Type         string `json:"type"`
		LoginEnabled bool   `json:"login_enabled"`
		Count        int    `json:"count"`
		Present      []int  `json:"present"`
		Diff         bool   `json:"diff"`
	}
	acctMap := map[string]*acctAgg{}
	acctOrder := []string{}
	suspicious := []gin.H{}
	oldFormat := 0

	for ii, it := range items {
		hostsOut = append(hostsOut, hostCol{ItemID: it.ID, HostName: it.HostName, HostIP: it.HostIP})
		rows := parseAccountsCSV(it.Content)
		if len(rows) == 0 {
			oldFormat++
			continue
		}
		uidUsers := map[int][]string{}
		for _, r := range rows {
			agg, ok := acctMap[r.Username]
			if !ok {
				agg = &acctAgg{Username: r.Username, UID: r.UID, Type: r.Type, LoginEnabled: r.LoginEnabled, Present: []int{}}
				acctMap[r.Username] = agg
				acctOrder = append(acctOrder, r.Username)
			}
			agg.Present = append(agg.Present, ii)
			agg.Count++
			if r.UID == 0 && r.Username != "root" {
				suspicious = append(suspicious, gin.H{"host": it.HostName, "username": r.Username, "reason": "uid0"})
			}
			if r.Type == "system" && r.LoginEnabled {
				suspicious = append(suspicious, gin.H{"host": it.HostName, "username": r.Username, "reason": "system_login"})
			}
			uidUsers[r.UID] = append(uidUsers[r.UID], r.Username)
		}
		for uid, users := range uidUsers {
			if len(users) > 1 {
				for _, u := range users {
					suspicious = append(suspicious, gin.H{"host": it.HostName, "username": u, "reason": "dup_uid", "uid": uid})
				}
			}
		}
	}

	accounts := make([]*acctAgg, 0, len(acctOrder))
	for _, name := range acctOrder {
		a := acctMap[name]
		a.Diff = a.Count < len(items)
		sort.Slice(a.Present, func(i, j int) bool { return a.Present[i] < a.Present[j] })
		accounts = append(accounts, a)
	}
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].UID != accounts[j].UID {
			return accounts[i].UID < accounts[j].UID
		}
		return accounts[i].Username < accounts[j].Username
	})

	c.JSON(http.StatusOK, gin.H{
		"report":     report,
		"hosts":      hostsOut,
		"accounts":   accounts,
		"suspicious": suspicious,
		"old_format": len(items) > 0 && oldFormat == len(items),
	})
}
