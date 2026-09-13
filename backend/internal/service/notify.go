// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"fmt"
	"html"
	"strings"
	"time"

	"jnexus/internal/model"
)

// NotifyTaskFinished sends a result email per the SMTP settings after a task finishes (async, silent on failure)
func NotifyTaskFinished(taskID uint) {
	go func() {
		defer func() { recover() }()
		smtp := LoadSMTPSettings()
		if !smtp.Enabled || !smtp.Notify || len(smtp.Recipients) == 0 {
			return
		}
		var task model.Task
		if err := model.DB.First(&task, taskID).Error; err != nil {
			return
		}
		var results []model.TaskHostResult
		model.DB.Where("task_id = ?", taskID).Order("id").Find(&results)

		okCnt, failCnt := 0, 0
		for _, r := range results {
			if r.Status == "failed" {
				failCnt++
			} else if r.Status == "success" {
				okCnt++
			}
		}
		color := "#67c23a"
		if failCnt > 0 {
			color = "#f56c6c"
		}
		var rows strings.Builder
		for _, r := range results {
			st := html.EscapeString(r.Status)
			stColor := "#67c23a"
			if r.Status == "failed" {
				stColor = "#f56c6c"
			}
			dur := ""
			if r.FinishedAt != nil {
				dur = fmt.Sprintf("%.1fs", r.FinishedAt.Sub(r.CreatedAt).Seconds())
			}
			out := r.Output
			if len(out) > 1500 {
				out = out[:1500] + "\n...(truncated)"
			}
			rows.WriteString(fmt.Sprintf(
				"<tr><td>%s (%s)</td><td>%s</td><td style='color:%s'>%s</td><td>%d</td><td>%s</td></tr>",
				html.EscapeString(r.HostName), html.EscapeString(r.HostIP),
				html.EscapeString(r.OsUser), stColor, st, r.ExitCode, dur))
			if r.Status == "failed" && out != "" {
				rows.WriteString(fmt.Sprintf("<tr><td colspan='5' style='background:#fef0f0'><pre style='margin:4px 0;white-space:pre-wrap'>%s</pre></td></tr>",
					html.EscapeString(out)))
			}
		}
		subject := fmt.Sprintf("[JNexus] 任务 #%d %s — 成功 %d / 失败 %d", taskID, taskTypeText2(task.Type), okCnt, failCnt)
		body := fmt.Sprintf(`
<h3>任务 #%d 已完成</h3>
<p>类型: %s &nbsp; 操作人: %s &nbsp; 时间: %s<br/>
结果: <b style="color:%s">成功 %d / 失败 %d / 共 %d</b></p>
<pre style="background:#f5f7fa;padding:8px;font-size:12px">%s</pre>
<table border="1" cellpadding="6" cellspacing="0" style="border-collapse:collapse;font-size:13px">
<tr style="background:#f5f7fa"><th>主机</th><th>OS账号</th><th>状态</th><th>退出码</th><th>耗时</th></tr>
%s
</table>
<p style="color:#909399;font-size:12px">由 JNexus 自动发送 · By JJ Zhang Version 1.0</p>`,
			taskID, taskTypeText2(task.Type), html.EscapeString(task.Operator),
			time.Now().Format("2006-01-02 15:04:05"), color, okCnt, failCnt, len(results),
			html.EscapeString(task.Params), rows.String())

		_ = SendMail(smtp, smtp.Recipients, subject, body)
	}()
}

func taskTypeText2(t string) string {
	switch t {
	case "command":
		return "命令执行"
	case "script":
		return "脚本执行"
	case "file":
		return "文件分发"
	case "release":
		return "应用发布"
	case "cred":
		return "批量添加账号"
	}
	return t
}
