// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"autoops/internal/model"
)

// ExportTask 任务输出汇总导出：?format=log（汇总日志）| csv（表格）
func ExportTask(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	u := currentUser(c)
	var task model.Task
	if err := model.DB.First(&task, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	// 范围：非管理员/审计员只能导出本人任务
	if !u.IsAdmin() && u.Role != model.RoleAuditor && task.Operator != u.Username {
		c.JSON(http.StatusForbidden, gin.H{"error": "只能导出本人发起的任务"})
		return
	}
	var results []model.TaskHostResult
	model.DB.Where("task_id = ?", id).Order("id").Find(&results)

	stamp := time.Now().Format("20060102_150405")
	format := c.DefaultQuery("format", "log")

	if format == "csv" {
		var sb strings.Builder
		sb.WriteString("\uFEFF") // Excel BOM
		sb.WriteString("主机名,IP,OS账号,状态,退出码,开始时间,结束时间,耗时(秒),输出\n")
		for _, r := range results {
			dur := ""
			if r.FinishedAt != nil {
				dur = fmt.Sprintf("%.1f", r.FinishedAt.Sub(r.CreatedAt).Seconds())
			}
			row := []string{r.HostName, r.HostIP, r.OsUser, r.Status, strconv.Itoa(r.ExitCode),
				r.CreatedAt.Format("2006-01-02 15:04:05"),
				timeFmt(r.FinishedAt), dur, r.Output}
			for i := range row {
				row[i] = csvEscape(row[i])
			}
			sb.WriteString(strings.Join(row, ",") + "\n")
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=task-%d-%s.csv", id, stamp))
		c.Data(http.StatusOK, "application/octet-stream", []byte(sb.String()))
		return
	}

	// 汇总日志
	var sb strings.Builder
	okCnt, failCnt, runCnt := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "success":
			okCnt++
		case "failed":
			failCnt++
		default:
			runCnt++
		}
	}
	sb.WriteString("================ AutoOps 任务 #" + strconv.Itoa(id) + " ================\n")
	sb.WriteString("类型: " + taskTypeText(task.Type) + "    操作人: " + task.Operator + "    发起时间: " + task.CreatedAt.Format("2006-01-02 15:04:05") + "\n")
	sb.WriteString("参数: " + task.Params + "\n")
	sb.WriteString(fmt.Sprintf("目标: %d 台    成功: %d    失败: %d    进行中: %d\n", len(results), okCnt, failCnt, runCnt))
	sb.WriteString("\n")
	for _, r := range results {
		dur := ""
		if r.FinishedAt != nil {
			dur = fmt.Sprintf("  耗时 %.1fs", r.FinishedAt.Sub(r.CreatedAt).Seconds())
		}
		sb.WriteString(fmt.Sprintf("---------- [%s] %s (%s)  账号=%s  exit=%d%s ----------\n",
			strings.ToUpper(statusText(r.Status)), r.HostName, r.HostIP, r.OsUser, r.ExitCode, dur))
		if strings.TrimSpace(r.Output) == "" {
			sb.WriteString("(无输出)\n")
		} else {
			sb.WriteString(r.Output)
			if !strings.HasSuffix(r.Output, "\n") {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}
	sb.WriteString("================ 导出时间 " + time.Now().Format("2006-01-02 15:04:05") + " ================\n")

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=task-%d-%s.log", id, stamp))
	c.Data(http.StatusOK, "application/octet-stream", []byte(sb.String()))
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func timeFmt(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func taskTypeText(t string) string {
	switch t {
	case "command":
		return "命令"
	case "script":
		return "脚本"
	case "file":
		return "文件分发"
	case "release":
		return "发布"
	}
	return t
}

func statusText(s string) string {
	switch s {
	case "success":
		return "成功"
	case "failed":
		return "失败"
	case "running":
		return "进行中"
	case "pending":
		return "待执行"
	}
	return s
}

var _ = utf8.RuneLen
