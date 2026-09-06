// JNexus 运维平台 — By JJ Zhang, Version 1.0
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"autoops/internal/model"
	"autoops/internal/service"
)

var (
	errCronType    = &cronError{"类型必须是 command 或 script"}
	errCronCommand = &cronError{"命令内容必填"}
	errCronScript  = &cronError{"脚本类型需要选择脚本"}
)

type cronError struct{ msg string }

func (e *cronError) Error() string { return e.msg }

func jsonMarshal(v any) ([]byte, error)      { return json.Marshal(v) }
func jsonUnmarshal(data string, v any) error { return json.Unmarshal([]byte(data), v) }

type cronJobReq struct {
	Name         string `json:"name" binding:"required"`
	Type         string `json:"type" binding:"required"` // command / script
	Command      string `json:"command"`
	ScriptID     *uint  `json:"script_id"`
	ScriptArgs   string `json:"script_args"`
	HostIDs      []uint `json:"host_ids"`
	GroupID      *uint  `json:"group_id"`
	IPs          string `json:"ips"`
	CredentialID *uint  `json:"credential_id"`
	CronExpr     string `json:"cron_expr" binding:"required"`
	TimeoutSec   int    `json:"timeout_sec"`
	Concurrency  int    `json:"concurrency"`
	Enabled      *bool  `json:"enabled"`
}

func (r *cronJobReq) validate() error {
	if r.Type != "command" && r.Type != "script" {
		return errCronType
	}
	if r.Type == "command" && r.Command == "" {
		return errCronCommand
	}
	if r.Type == "script" && r.ScriptID == nil {
		return errCronScript
	}
	return service.ValidateCronExpr(r.CronExpr)
}

func (r *cronJobReq) toJob(j *model.CronJob) error {
	j.Name = r.Name
	j.Type = r.Type
	j.Command = r.Command
	j.ScriptID = r.ScriptID
	j.ScriptArgs = r.ScriptArgs
	j.GroupID = r.GroupID
	j.IPs = r.IPs
	j.CredentialID = r.CredentialID
	j.CronExpr = r.CronExpr
	if r.TimeoutSec > 0 {
		j.TimeoutSec = r.TimeoutSec
	}
	if r.Concurrency > 0 {
		j.Concurrency = r.Concurrency
	}
	if r.Enabled != nil {
		j.Enabled = *r.Enabled
	}
	if len(r.HostIDs) > 0 {
		b, _ := jsonMarshal(r.HostIDs)
		j.HostIDs = string(b)
	} else {
		j.HostIDs = ""
	}
	return nil
}

// ListCrons 计划任务列表（含下次/上次运行）
func ListCrons(c *gin.Context) {
	var jobs []model.CronJob
	model.DB.Order("id DESC").Find(&jobs)
	// 修正过期的下次运行时间展示
	now := time.Now()
	out := make([]gin.H, 0, len(jobs))
	for _, j := range jobs {
		next := j.NextRunAt
		if j.Enabled {
			if n, err := service.NextCronTime(j.CronExpr, now); err == nil {
				next = n
			}
		}
		hostCnt := 0
		var hostIDs []uint
		if len(j.HostIDs) > 0 {
			if jsonUnmarshal(j.HostIDs, &hostIDs) == nil {
				hostCnt = len(hostIDs)
			}
		}
		out = append(out, gin.H{
			"id": j.ID, "name": j.Name, "type": j.Type,
			"command": j.Command, "script_id": j.ScriptID, "script_args": j.ScriptArgs,
			"cron_expr": j.CronExpr, "enabled": j.Enabled,
			"host_ids": hostIDs, "host_count": hostCnt, "ips": j.IPs, "group_id": j.GroupID,
			"credential_id": j.CredentialID, "timeout_sec": j.TimeoutSec, "concurrency": j.Concurrency,
			"next_run_at": next, "last_run_at": j.LastRunAt,
			"last_task_id": j.LastTaskID,
			"created_by":   j.CreatedBy, "created_at": j.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, out)
}

func CreateCron(c *gin.Context) {
	var req cronJobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误（名称/类型/cron 表达式必填）"})
		return
	}
	if err := req.validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u := currentUser(c)
	job := model.CronJob{CreatedBy: u.Username, Enabled: true}
	if err := req.toJob(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if next, err := service.NextCronTime(job.CronExpr, time.Now()); err == nil {
		job.NextRunAt = next
	}
	if err := model.DB.Create(&job).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "计划任务名已存在"})
		return
	}
	c.JSON(http.StatusOK, job)
}

func UpdateCron(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var job model.CronJob
	if err := model.DB.First(&job, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "计划任务不存在"})
		return
	}
	var req cronJobReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := req.validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := req.toJob(&job); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if next, err := service.NextCronTime(job.CronExpr, time.Now()); err == nil {
		job.NextRunAt = next
	}
	model.DB.Save(&job)
	c.JSON(http.StatusOK, job)
}

func DeleteCron(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	model.DB.Delete(&model.CronJob{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ToggleCron 启用/禁用（禁用时清除下次运行时间）
func ToggleCron(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var job model.CronJob
	if err := model.DB.First(&job, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "计划任务不存在"})
		return
	}
	enabled := !job.Enabled
	updates := map[string]any{"enabled": enabled}
	if enabled {
		if next, err := service.NextCronTime(job.CronExpr, time.Now()); err == nil {
			updates["next_run_at"] = next
		}
	} else {
		updates["next_run_at"] = nil
	}
	model.DB.Model(&job).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"ok": true, "enabled": enabled})
}

// RunCronNow 立即执行一次
func RunCronNow(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var job model.CronJob
	if err := model.DB.First(&job, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "计划任务不存在"})
		return
	}
	u := currentUser(c)
	operator := &model.User{Username: u.Username, Role: model.RoleAdmin, Status: 1}
	req := service.ExecRequest{
		Command:      job.Command,
		ScriptArgs:   job.ScriptArgs,
		TimeoutSec:   job.TimeoutSec,
		Concurrency:  job.Concurrency,
		CredentialID: job.CredentialID,
	}
	if job.Type == "script" {
		req.ScriptID = job.ScriptID
		req.Command = ""
	}
	if len(job.HostIDs) > 0 {
		var ids []uint
		if jsonUnmarshal(job.HostIDs, &ids) == nil {
			req.HostIDs = ids
		}
	}
	if job.GroupID != nil {
		req.GroupID = job.GroupID
	}
	if job.IPs != "" {
		req.IPs = []string{job.IPs}
	}
	taskID, _, err := service.StartBatchExec(operator, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	now := time.Now()
	model.DB.Model(&job).Updates(map[string]any{"last_run_at": now, "last_task_id": taskID})
	model.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("cron_job_id", job.ID)
	c.JSON(http.StatusOK, gin.H{"task_id": taskID})
}

// CronHistory 某计划任务的执行历史
func CronHistory(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var tasks []model.Task
	model.DB.Where("cron_job_id = ?", id).Order("id DESC").Limit(50).Find(&tasks)
	c.JSON(http.StatusOK, tasks)
}
