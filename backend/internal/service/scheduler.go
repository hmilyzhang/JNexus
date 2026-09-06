// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"
	"fmt"
	"time"

	cronlib "github.com/robfig/cron/v3"

	"autoops/internal/model"
)

// ValidateCronExpr 校验 cron 表达式（5 段标准格式）
func ValidateCronExpr(expr string) error {
	if _, err := cronlib.ParseStandard(expr); err != nil {
		return fmt.Errorf("非法 cron 表达式: %w", err)
	}
	return nil
}

// NextCronTime 计算表达式在 base 之后的下一次触发时间
func NextCronTime(expr string, base time.Time) (*time.Time, error) {
	sched, err := cronlib.ParseStandard(expr)
	if err != nil {
		return nil, err
	}
	next := sched.Next(base)
	return &next, nil
}

// StartScheduler 启动计划任务调度循环（每 20 秒扫描一次到期任务）
func StartScheduler() {
	go func() {
		for {
			tick()
			time.Sleep(20 * time.Second)
		}
	}()
}

func tick() {
	defer func() { recover() }()
	go ScanDueRotations() // 密码轮换扫描
	var jobs []model.CronJob
	if err := model.DB.Where("enabled = ?", true).Find(&jobs).Error; err != nil {
		return
	}
	now := time.Now()
	for _, job := range jobs {
		next, err := NextCronTime(job.CronExpr, now)
		if err != nil {
			continue // 表达式非法，跳过
		}
		// 首次见到：只设置下次触发时间，不立即执行
		if job.NextRunAt == nil {
			model.DB.Model(&job).Update("next_run_at", next)
			continue
		}
		if now.Before(*job.NextRunAt) {
			continue // 未到期
		}
		// 到期：立即更新下次触发时间（防止重复触发），异步执行
		model.DB.Model(&job).Updates(map[string]any{
			"next_run_at": next,
			"last_run_at": now,
		})
		go executeCronJob(job, now)
	}
}

// executeCronJob 触发一次计划任务（以创建者身份执行，留痕于任务记录）
func executeCronJob(job model.CronJob, firedAt time.Time) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("[cron] panic:", r)
		}
	}()

	// 重新读取（期间可能被修改/禁用/删除）
	var fresh model.CronJob
	if err := model.DB.First(&fresh, job.ID).Error; err != nil || !fresh.Enabled {
		return
	}

	// 以创建者身份执行（合成用户，权限全通过；操作人显示为创建者，便于审计）
	operator := &model.User{Username: fresh.CreatedBy, Role: model.RoleAdmin, Status: 1}

	req := ExecRequest{
		Command:      fresh.Command,
		ScriptArgs:   fresh.ScriptArgs,
		TimeoutSec:   fresh.TimeoutSec,
		Concurrency:  fresh.Concurrency,
		CredentialID: fresh.CredentialID,
	}
	if fresh.Type == "script" {
		req.ScriptID = fresh.ScriptID
		req.Command = ""
	}
	if len(fresh.HostIDs) > 0 {
		var ids []uint
		if json.Unmarshal([]byte(fresh.HostIDs), &ids) == nil {
			req.HostIDs = ids
		}
	}
	if fresh.GroupID != nil {
		req.GroupID = fresh.GroupID
	}
	if fresh.IPs != "" {
		req.IPs = []string{fresh.IPs}
	}

	taskID, _, err := StartBatchExec(operator, req)
	if err != nil {
		// 目标不可用等情况：记录一条失败任务便于排查
		task := model.Task{
			Type: model.TaskCommand, Operator: fresh.CreatedBy,
			Params: fmt.Sprintf(`{"cron":"%s","error":"%v"}`, fresh.Name, err),
			Status: "failed", CronJobID: &fresh.ID,
			FinishedAt: ptrTime(time.Now()),
		}
		model.DB.Create(&task)
		model.DB.Model(&fresh).Update("last_task_id", task.ID)
		return
	}
	model.DB.Model(&model.Task{}).Where("id = ?", taskID).Update("cron_job_id", fresh.ID)
	model.DB.Model(&fresh).Update("last_task_id", taskID)
}

func ptrTime(t time.Time) *time.Time { return &t }
