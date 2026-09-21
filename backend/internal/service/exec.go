// JNexus Ops Platform — By JJ Zhang, Version 1.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/sshpool"
	"jnexus/internal/ws"
)

const maxOutputSize = 512 * 1024 // per-host output cap 512KB

// ExecRequest batch execution request
type ExecRequest struct {
	HostIDs      []uint   `json:"host_ids"`
	GroupID      *uint    `json:"group_id"`
	IPs          []string `json:"ips"`           // multiple IPs, comma-separated input
	CredentialID *uint    `json:"credential_id"` // specify OS account (optional; defaults to each host's default usable account)
	Command      string   `json:"command"`
	ScriptID     *uint    `json:"script_id"`
	ScriptArgs   string   `json:"script_args"`
	TimeoutSec   int      `json:"timeout_sec"`
	Concurrency  int      `json:"concurrency"`
}

// StartBatchExec creates a task and concurrently runs the command/script; returns the task id (or interception reasons)
func StartBatchExec(operator *model.User, req ExecRequest) (uint, []string, error) {
	command := req.Command
	// Windows variant: command mode runs the same text via WinRM/PowerShell; script
	// mode uses the script's PowerShell version (empty = Windows hosts are skipped)
	winCommand := command
	if req.ScriptID != nil {
		var sc model.Script
		if err := model.DB.First(&sc, *req.ScriptID).Error; err != nil {
			return 0, nil, fmt.Errorf("脚本不存在")
		}
		command = sc.Content
		winCommand = sc.ContentPS
		if strings.TrimSpace(req.ScriptArgs) != "" {
			command += "\n" + req.ScriptArgs
			winCommand += "\n" + req.ScriptArgs
		}
	}
	if strings.TrimSpace(command) == "" {
		return 0, nil, fmt.Errorf("命令不能为空")
	}

	// Dangerous command interception
	if hits := CheckDanger(command); len(hits) > 0 {
		return 0, hits, fmt.Errorf("危险命令已被拦截: %s", strings.Join(hits, "、"))
	}
	if winCommand != command {
		if hits := CheckDanger(winCommand); len(hits) > 0 {
			return 0, hits, fmt.Errorf("危险命令已被拦截(Windows 版本): %s", strings.Join(hits, "、"))
		}
	}

	hosts, err := resolveHosts(operator, req.HostIDs, req.GroupID, req.IPs)
	if err != nil {
		return 0, nil, err
	}
	if len(hosts) == 0 {
		return 0, nil, fmt.Errorf("未选择任何有权限的目标主机")
	}

	// When an OS account is specified, verify usage rights at submission time to avoid failing after the task is created
	if req.CredentialID != nil {
		var cred model.HostCredential
		if err := model.DB.First(&cred, *req.CredentialID).Error; err != nil {
			return 0, nil, fmt.Errorf("OS 账号不存在")
		}
		var credHost model.Host
		if err := model.DB.First(&credHost, cred.HostID).Error; err != nil {
			return 0, nil, fmt.Errorf("OS 账号所属主机不存在")
		}
		if _, err := ResolveCredential(operator, &credHost, req.CredentialID); err != nil {
			return 0, nil, err
		}
	}

	conc := req.Concurrency
	if conc <= 0 {
		conc = 10
	}
	timeout := req.TimeoutSec
	if timeout <= 0 {
		timeout = 300
	}

	params, _ := json.Marshal(map[string]any{
		"command": command, "timeout_sec": timeout, "concurrency": conc,
	})
	task := model.Task{
		Type:     model.TaskCommand,
		Operator: operator.Username,
		Params:   string(params),
		Status:   "running",
	}
	if req.ScriptID != nil {
		task.Type = model.TaskScript
	}
	if err := model.DB.Create(&task).Error; err != nil {
		return 0, nil, err
	}

	// Initialize result rows
	results := make([]model.TaskHostResult, 0, len(hosts))
	for _, h := range hosts {
		results = append(results, model.TaskHostResult{
			TaskID: task.ID, HostID: h.ID, HostIP: h.IP, HostName: h.Name, Status: "pending",
		})
	}
	if err := model.DB.Create(&results).Error; err != nil {
		return 0, nil, err
	}

	go runTask(operator, req.CredentialID, task.ID, command, winCommand, results, conc, time.Duration(timeout)*time.Second)
	return task.ID, nil, nil
}

// resolveHosts resolves target hosts from host IDs / group / IP list and applies permission filtering
func resolveHosts(operator *model.User, hostIDs []uint, groupID *uint, ips []string) ([]model.Host, error) {
	var hosts []model.Host
	if len(hostIDs) > 0 {
		if err := model.DB.Preload("Group").Where("id IN ?", hostIDs).Find(&hosts).Error; err != nil {
			return nil, err
		}
	} else if groupID != nil {
		// Include all descendant groups (multi-level tree cascade)
		if err := model.DB.Preload("Group").Where("group_id IN ?", GroupAndDescendants(*groupID)).Find(&hosts).Error; err != nil {
			return nil, err
		}
	} else if len(ips) > 0 {
		// The frontend already splits by comma; do a fault-tolerant split here as well
		var flat []string
		for _, s := range ips {
			for _, p := range strings.Split(s, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					flat = append(flat, p)
				}
			}
		}
		if len(flat) == 0 {
			return nil, fmt.Errorf("IP 列表为空")
		}
		if err := model.DB.Preload("Group").Where("ip IN ?", flat).Find(&hosts).Error; err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("请选择目标主机或分组")
	}
	// Data-level permission filtering
	allowed := hosts[:0]
	for i := range hosts {
		// Host-level permission (personal grant / user-group linked hosts) or a usable OS account on that host (user-group linked credentials)
		if CanExecHost(operator, hosts[i].ID, hosts[i].GroupID) || len(UsableCredentials(operator, &hosts[i])) > 0 {
			allowed = append(allowed, hosts[i])
		}
	}
	return allowed, nil
}

// CanExecHost whether the user can execute on the given host:
// admin is always allowed; ops/publisher gain access via "personal group grants" or "user groups (directly linked hosts / linked host groups)"
func CanExecHost(user *model.User, hostID uint, groupID *uint) bool {
	if user.IsAdmin() {
		return true
	}
	if user.Role != model.RoleOps && user.Role != model.RolePublisher {
		return false
	}
	// Personal grant: the host's group (including ancestor group cascade)
	if groupID != nil {
		chain := GroupAncestors(*groupID)
		var cnt int64
		model.DB.Model(&model.UserHostGroup{}).
			Where("user_id = ? AND group_id IN ? AND can_exec = ?", user.ID, chain, true).
			Count(&cnt)
		if cnt > 0 {
			return true
		}
	}
	// Hosts directly linked by user groups
	var cnt int64
	model.DB.Table("user_group_hosts ug_h").
		Joins("JOIN user_group_members ug_m ON ug_m.user_group_id = ug_h.user_group_id").
		Where("ug_m.user_id = ? AND ug_h.host_id = ?", user.ID, hostID).
		Count(&cnt)
	if cnt > 0 {
		return true
	}
	// Host groups linked by user groups (applies if any ancestor of the host's group matches)
	if groupID != nil {
		chain := GroupAncestors(*groupID)
		model.DB.Table("user_group_host_groups ug_g").
			Joins("JOIN user_group_members ug_m ON ug_m.user_group_id = ug_g.user_group_id").
			Where("ug_m.user_id = ? AND ug_g.host_group_id IN ?", user.ID, chain).
			Count(&cnt)
		if cnt > 0 {
			return true
		}
	}
	return false
}

// runTask executes the task concurrently, streaming output in real time.
// command runs on Linux (SSH/bash); winCommand runs on Windows (WinRM/PowerShell) —
// an empty winCommand means script mode without a Windows version: those hosts are
// recorded as skipped instead of failing with PowerShell syntax errors.
func runTask(operator *model.User, reqCredID *uint, taskID uint, command, winCommand string, results []model.TaskHostResult, concurrency int, timeout time.Duration) {
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	remain := len(results)

	pushTaskStatus := func() {
		mu.Lock()
		remain--
		left := remain
		mu.Unlock()
		if left == 0 {
			finished := time.Now()
			status := "done"
			var failCnt int64
			model.DB.Model(&model.TaskHostResult{}).Where("task_id = ? AND status = ?", taskID, "failed").Count(&failCnt)
			if failCnt > 0 {
				status = "failed"
			}
			model.DB.Model(&model.Task{}).Where("id = ?", taskID).
				Updates(map[string]any{"status": status, "finished_at": finished})
			ws.H.Broadcast(taskTopic(taskID), map[string]any{"type": "task_done", "status": status})
			NotifyTaskFinished(taskID)
		}
	}

	for i := range results {
		wg.Add(1)
		go func(res model.TaskHostResult) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			host := model.Host{ID: res.HostID, IP: res.HostIP, Name: res.HostName}
			model.DB.First(&host, res.HostID)

			model.DB.Model(&res).Update("status", "running")
			ws.H.Broadcast(taskTopic(taskID), map[string]any{
				"type": "status", "result_id": res.ID, "host_id": res.HostID, "status": "running",
			})

			cred, cerr := ResolveCredential(operator, &host, reqCredID)
			if cerr != nil {
				finishResult(res.ID, -1, cerr.Error(), "failed")
				pushTaskStatus()
				return
			}
			// Windows host: run via WinRM (no streaming output; pushed in one shot after completion)
			if IsWindows(&host) {
				if winCommand == "" {
					// script mode without a PowerShell version: skip, don't fail
					const skipMsg = "该脚本未提供 Windows (PowerShell) 版本，已跳过"
					model.DB.Model(&model.TaskHostResult{}).Where("id = ?", res.ID).
						Updates(map[string]any{"status": "skipped", "output": skipMsg, "finished_at": time.Now()})
					ws.H.Broadcast(taskTopic(taskID), map[string]any{
						"type": "output", "result_id": res.ID, "host_id": res.HostID, "text": skipMsg,
					})
					ws.H.Broadcast(taskTopic(taskID), map[string]any{"type": "status", "result_id": res.ID, "status": "skipped"})
					pushTaskStatus()
					return
				}
				model.DB.Model(&model.TaskHostResult{}).Where("id = ?", res.ID).Update("os_user", cred.Username)
				ws.H.Broadcast(taskTopic(taskID), map[string]any{"type": "os_user", "result_id": res.ID, "os_user": cred.Username})
				pass := ""
				if cred.AuthType == "password" {
					p, derr := pkg.Decrypt(cred.Password)
					if derr != nil {
						finishResult(res.ID, -1, "凭据解密失败: "+derr.Error(), "failed")
						pushTaskStatus()
						return
					}
					pass = p
				}
				out, code, werr := WinRMRun(&host, cred.Username, pass, winCommand, int(timeout.Seconds()))
				status := "success"
				if werr != nil {
					out += "\n[错误] " + werr.Error()
					status = "failed"
					code = -1
				} else if code != 0 {
					status = "failed"
				}
				ws.H.Broadcast(taskTopic(taskID), map[string]any{
					"type": "output", "result_id": res.ID, "host_id": res.HostID, "text": out,
				})
				finishResult(res.ID, code, out, status)
				// dual-write the Windows task result into the task_logs stream too
				// (parity with the SSH branch)
				var taskType, operator string
				var task model.Task
				if model.DB.Select("type", "operator").First(&task, taskID).Error == nil {
					taskType, operator = task.Type, task.Operator
				}
				OOPushTaskLog(taskID, taskType, operator, res.HostName, res.OsUser, status, code, out)
				pushTaskStatus()
				return
			}

			cli, err := sshpool.ClientForCredential(&host, cred)
			if err != nil {
				finishResult(res.ID, -1, "连接失败: "+err.Error(), "failed")
				pushTaskStatus()
				return
			}
			defer cli.Close()
			model.DB.Model(&model.TaskHostResult{}).Where("id = ?", res.ID).Update("os_user", cred.Username)
			ws.H.Broadcast(taskTopic(taskID), map[string]any{"type": "os_user", "result_id": res.ID, "os_user": cred.Username})

			var buf strings.Builder
			var wmu sync.Mutex
			onOut := func(chunk string) {
				wmu.Lock()
				if buf.Len() < maxOutputSize {
					buf.WriteString(chunk)
				}
				wmu.Unlock()
				ws.H.Broadcast(taskTopic(taskID), map[string]any{
					"type": "output", "result_id": res.ID, "host_id": res.HostID, "text": chunk,
				})
			}

			code, err := sshpool.RunCommand(context.Background(), cli, command, timeout, onOut)
			status := "success"
			out := buf.String()
			if err != nil {
				out += "\n[错误] " + err.Error()
				status = "failed"
				if code == 0 {
					code = -1
				}
			} else if code != 0 {
				status = "failed"
			}
			finishResult(res.ID, code, out, status)
			var taskType, operator string
			var task model.Task
			if model.DB.Select("type", "operator").First(&task, taskID).Error == nil {
				taskType, operator = task.Type, task.Operator
			}
			OOPushTaskLog(taskID, taskType, operator, res.HostName, res.OsUser, status, code, out)
			pushTaskStatus()
		}(results[i])
	}
	wg.Wait()
}

func finishResult(id uint, code int, out, status string) {
	model.DB.Model(&model.TaskHostResult{}).Where("id = ?", id).
		Updates(map[string]any{
			"exit_code": code, "output": out, "status": status, "finished_at": time.Now(),
		})
	ws.H.Broadcast("results", map[string]any{"result_id": id, "status": status})
}

func taskTopic(taskID uint) string { return fmt.Sprintf("task-%d", taskID) }

// ProbeHosts probes host connectivity concurrently and updates their status
func ProbeHosts(hostIDs []uint) int {
	var hosts []model.Host
	q := model.DB
	if len(hostIDs) > 0 {
		q = q.Where("id IN ?", hostIDs)
	}
	if err := q.Find(&hosts).Error; err != nil {
		return 0
	}
	sem := make(chan struct{}, 50)
	var wg sync.WaitGroup
	online := 0
	var mu sync.Mutex
	for _, h := range hosts {
		wg.Add(1)
		go func(h model.Host) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			status := "offline"
			if err := sshpool.Probe(h.IP, h.Port, 5*time.Second); err == nil {
				status = "online"
				mu.Lock()
				online++
				mu.Unlock()
			}
			model.DB.Model(&model.Host{}).Where("id = ?", h.ID).
				Updates(map[string]any{"status": status, "last_seen": time.Now()})
		}(h)
	}
	wg.Wait()
	return online
}

func init() {
	log.SetFlags(log.LstdFlags)
}
