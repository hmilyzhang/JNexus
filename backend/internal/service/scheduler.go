// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"
	"fmt"
	"time"

	cronlib "github.com/robfig/cron/v3"

	"jnexus/internal/model"
)

// ValidateCronExpr validates a cron expression (5-field standard format)
func ValidateCronExpr(expr string) error {
	if _, err := cronlib.ParseStandard(expr); err != nil {
		return fmt.Errorf("非法 cron 表达式: %w", err)
	}
	return nil
}

// NextCronTime computes the next fire time of the expression after base
func NextCronTime(expr string, base time.Time) (*time.Time, error) {
	sched, err := cronlib.ParseStandard(expr)
	if err != nil {
		return nil, err
	}
	next := sched.Next(base)
	return &next, nil
}

// StartScheduler starts the cron job scheduling loop (scans due jobs every 20 seconds)
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
	go ScanDueRotations() // password rotation scan
	var jobs []model.CronJob
	if err := model.DB.Where("enabled = ?", true).Find(&jobs).Error; err != nil {
		return
	}
	now := time.Now()
	for _, job := range jobs {
		next, err := NextCronTime(job.CronExpr, now)
		if err != nil {
			continue // invalid expression, skip
		}
		// first sighting: only set the next fire time, do not run immediately
		if job.NextRunAt == nil {
			model.DB.Model(&job).Update("next_run_at", next)
			continue
		}
		if now.Before(*job.NextRunAt) {
			continue // not due yet
		}
		// due: update the next fire time immediately (prevents duplicate firing), run async
		model.DB.Model(&job).Updates(map[string]any{
			"next_run_at": next,
			"last_run_at": now,
		})
		go executeCronJob(job, now)
	}
}

// executeCronJob fires a cron job once (executed as the creator, recorded in the task log)
func executeCronJob(job model.CronJob, firedAt time.Time) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("[cron] panic:", r)
		}
	}()

	// re-read (it may have been modified/disabled/deleted in the meantime)
	var fresh model.CronJob
	if err := model.DB.First(&fresh, job.ID).Error; err != nil || !fresh.Enabled {
		return
	}

	// execute as the creator (synthesized user, all permissions pass; operator shown as creator for auditability)
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
		// targets unavailable and similar cases: record a failed task for troubleshooting
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
