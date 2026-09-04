// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"autoops/internal/model"
	"autoops/internal/service"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（模板必选）"})
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
		c.JSON(http.StatusNotFound, gin.H{"error": "报告不存在"})
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
		sb.WriteString("主机名,IP,状态,错误,采集内容\n")
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
	sb.WriteString("================ AutoOps 采集报告 #" + strconv.Itoa(int(report.ID)) + " ================\n")
	sb.WriteString("模板: " + tplName + "    操作人: " + report.Operator + "    时间: " + report.CreatedAt.Format("2006-01-02 15:04:05") + "\n")
	sb.WriteString(fmt.Sprintf("目标: %d 台\n\n", len(items)))
	for _, it := range items {
		st := "成功"
		if it.Status == "failed" {
			st = "失败"
		}
		sb.WriteString(fmt.Sprintf("---------- [%s] %s (%s) ----------\n", st, it.HostName, it.HostIP))
		if it.Error != "" {
			sb.WriteString("错误: " + it.Error + "\n")
		}
		if strings.TrimSpace(it.Content) == "" {
			sb.WriteString("(无输出)\n")
		} else {
			sb.WriteString(it.Content)
			if !strings.HasSuffix(it.Content, "\n") {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("================ 导出时间 " + time.Now().Format("2006-01-02 15:04:05") + " ================\n")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=report-%d-%s.log", int(report.ID), stamp))
	c.Data(http.StatusOK, "application/octet-stream", []byte(sb.String()))
}
