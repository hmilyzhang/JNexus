// JNexus Ops Platform — By JJ Zhang, Version 1.0

package service

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/sshpool"
	"jnexus/internal/ws"
)

// CreateReleaseRequest creates a release order
type CreateReleaseRequest struct {
	AppID       uint   `json:"app_id"`
	PackageFile string `json:"package_file"` // file name already uploaded to the server staging area
	PackageName string `json:"package_name"`
}

var releaseSteps = []string{"stop", "backup", "upload", "start", "health"}

// StartRelease creates a release order and starts the pipeline
func StartRelease(operator *model.User, req CreateReleaseRequest) (uint, error) {
	var app model.Application
	if err := model.DB.Preload("AppHosts.Host").First(&app, req.AppID).Error; err != nil {
		return 0, fmt.Errorf("应用不存在")
	}
	if len(app.AppHosts) == 0 {
		return 0, fmt.Errorf("应用未绑定任何主机")
	}
	if req.PackageFile == "" {
		return 0, fmt.Errorf("请先上传发布包")
	}
	if !UserCanDeployApp(operator, req.AppID) {
		return 0, fmt.Errorf("无该应用的发布权限")
	}

	rel := model.Release{
		AppID: app.ID, AppName: app.Name, Operator: operator.Username,
		PackageFile: req.PackageFile, PackageName: req.PackageName, Status: "running",
	}
	if err := model.DB.Create(&rel).Error; err != nil {
		return 0, err
	}
	for _, ah := range app.AppHosts {
		model.DB.Create(&model.ReleaseItem{
			ReleaseID: rel.ID, HostID: ah.HostID, HostIP: ah.Host.IP, HostName: ah.Host.Name,
			Step: "pending", Status: "pending",
		})
	}

	task := model.Task{
		Type: model.TaskRelease, Operator: operator.Username, Status: "running",
		Params: model.JSONParams(map[string]any{"release_id": rel.ID, "app": app.Name}),
	}
	model.DB.Create(&task)

	go runRelease(rel.ID)
	return rel.ID, nil
}

// runRelease runs the release pipeline: concurrent across hosts, sequential steps within a single host
func runRelease(releaseID uint) {
	var rel model.Release
	if err := model.DB.First(&rel, releaseID).Error; err != nil {
		return
	}
	var items []model.ReleaseItem
	model.DB.Where("release_id = ?", releaseID).Find(&items)

	var wg sync.WaitGroup
	var mu sync.Mutex
	remain := len(items)

	sem := make(chan struct{}, 20)
	for _, item := range items {
		wg.Add(1)
		go func(item model.ReleaseItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					updateItem(item.ID, "health", "failed", fmt.Sprintf("panic: %v", r))
				}
				mu.Lock()
				remain--
				mu.Unlock()
			}()
			runHostPipeline(&rel, item)
		}(item)
	}
	wg.Wait()

	// aggregate status
	var fails int64
	model.DB.Model(&model.ReleaseItem{}).Where("release_id = ? AND status = ?", releaseID, "failed").Count(&fails)
	status := "success"
	if fails > 0 {
		status = "failed"
	}
	model.DB.Model(&model.Release{}).Where("id = ?", releaseID).Update("status", status)
	ws.H.Broadcast(fmt.Sprintf("release-%d", releaseID), map[string]any{"type": "release_done", "status": status})
	NotifyTaskFinished(100000 + releaseID)
}

// runHostPipeline runs the step pipeline on a single host
func runHostPipeline(rel *model.Release, item model.ReleaseItem) {
	var ah model.AppHost
	if err := model.DB.Where("app_id = ? AND host_id = ?", rel.AppID, item.HostID).First(&ah).Error; err != nil {
		updateItem(item.ID, "stop", "failed", "未找到该主机的部署配置")
		return
	}
	host := model.Host{}
	model.DB.First(&host, item.HostID)

	cred, cerr := resolveReleaseCredential(&ah, &host)
	if cerr != nil {
		updateItem(item.ID, "stop", "failed", cerr.Error())
		return
	}
	cli, err := sshpool.ClientForCredential(&host, cred)
	if err != nil {
		updateItem(item.ID, "stop", "failed", "连接失败: "+err.Error())
		return
	}
	defer cli.Close()

	run := func(step, cmd string, timeout time.Duration) bool {
		if hits := CheckDanger(cmd); len(hits) > 0 {
			updateItem(item.ID, step, "failed", "危险命令拦截: "+strings.Join(hits, "、"))
			return false
		}
		var buf strings.Builder
		code, err := sshpool.RunCommand(context.Background(), cli, cmd, timeout, func(s string) { buf.WriteString(s) })
		if err != nil {
			updateItem(item.ID, step, "failed", "执行失败: "+err.Error()+"\n"+buf.String())
			return false
		}
		if code != 0 {
			updateItem(item.ID, step, "failed", fmt.Sprintf("退出码 %d\n%s", code, buf.String()))
			return false
		}
		updateItem(item.ID, step, "success", buf.String())
		return true
	}

	// 1. Stop service
	stopCmd := ah.StopCmd
	if stopCmd == "" {
		stopCmd = fmt.Sprintf("pkill -f '%s' || true; sleep 2; pgrep -f '%s' && kill -9 $(pgrep -f '%s') || true",
			ah.JarName, ah.JarName, ah.JarName)
	}
	if !run("stop", stopCmd, 2*time.Minute) {
		return
	}

	// 2. Back up the old version
	backupDir := ah.BackupDir
	if backupDir == "" {
		backupDir = path.Join(ah.DeployDir, "backup")
	}
	stamp := time.Now().Format("20060102150405")
	backupFile := fmt.Sprintf("%s.%s.bak", ah.JarName, stamp)
	backupCmd := fmt.Sprintf("mkdir -p '%s' && if [ -f '%s/%s' ]; then cp -a '%s/%s' '%s/%s'; fi",
		backupDir, ah.DeployDir, ah.JarName, ah.DeployDir, ah.JarName, backupDir, backupFile)
	if !run("backup", backupCmd, 5*time.Minute) {
		return
	}

	// 3. Upload the new package
	localPath := LocalUploadPath(rel.PackageFile)
	scliErr := uploadViaSSH(cli, localPath, ah.DeployDir, ah.JarName)
	if scliErr != nil {
		updateItem(item.ID, "upload", "failed", "上传失败: "+scliErr.Error())
		return
	}
	updateItem(item.ID, "upload", "success", fmt.Sprintf("已上传 %s -> %s/%s", rel.PackageName, ah.DeployDir, ah.JarName))

	// 4. Start service
	startCmd := ah.StartCmd
	if startCmd == "" {
		startCmd = fmt.Sprintf("cd '%s' && nohup java -jar '%s' > /dev/null 2>&1 &", ah.DeployDir, ah.JarName)
	}
	if !run("start", startCmd, 5*time.Minute) {
		return
	}

	// 5. Health check
	if ah.HealthCheckURL == "" {
		// no URL configured: check by whether the process exists
		checkCmd := fmt.Sprintf("sleep 3; pgrep -f '%s' >/dev/null && echo OK || echo NO", ah.JarName)
		var buf strings.Builder
		_, err := sshpool.RunCommand(context.Background(), cli, checkCmd, 2*time.Minute, func(s string) { buf.WriteString(s) })
		if err != nil || !strings.Contains(buf.String(), "OK") {
			updateItem(item.ID, "health", "failed", "启动后未检测到进程\n"+buf.String())
			return
		}
		updateItem(item.ID, "health", "success", "进程已启动")
	} else {
		if !healthCheck(ah.HealthCheckURL, 12, 5*time.Second) {
			updateItem(item.ID, "health", "failed", "健康检查未通过: "+ah.HealthCheckURL)
			return
		}
		updateItem(item.ID, "health", "success", "健康检查通过")
	}
}

// RollbackRelease rolls back: restores the most recent backup over the deploy directory and restarts
func RollbackRelease(operator *model.User, releaseID uint) (uint, error) {
	var rel model.Release
	if err := model.DB.First(&rel, releaseID).Error; err != nil {
		return 0, fmt.Errorf("发布单不存在")
	}
	if !UserCanDeployApp(operator, rel.AppID) {
		return 0, fmt.Errorf("无该应用的发布权限")
	}

	rollback := model.Release{
		AppID: rel.AppID, AppName: rel.AppName, Operator: operator.Username,
		PackageFile: "", PackageName: "(回滚到最近备份)", Status: "running",
	}
	if err := model.DB.Create(&rollback).Error; err != nil {
		return 0, err
	}
	var items []model.ReleaseItem
	model.DB.Where("release_id = ?", releaseID).Find(&items)
	for _, it := range items {
		model.DB.Create(&model.ReleaseItem{
			ReleaseID: rollback.ID, HostID: it.HostID, HostIP: it.HostIP, HostName: it.HostName,
			Step: "pending", Status: "pending",
		})
	}

	go runRollback(&rollback)
	return rollback.ID, nil
}

func runRollback(rel *model.Release) {
	var items []model.ReleaseItem
	model.DB.Where("release_id = ?", rel.ID).Find(&items)
	var wg sync.WaitGroup
	for _, item := range items {
		wg.Add(1)
		go func(item model.ReleaseItem) {
			defer wg.Done()
			var ah model.AppHost
			if err := model.DB.Where("app_id = ? AND host_id = ?", rel.AppID, item.HostID).First(&ah).Error; err != nil {
				updateItem(item.ID, "stop", "failed", "未找到部署配置")
				return
			}
			host := model.Host{}
			model.DB.First(&host, item.HostID)
			cred, cerr := resolveReleaseCredential(&ah, &host)
			if cerr != nil {
				updateItem(item.ID, "stop", "failed", cerr.Error())
				return
			}
			cli, err := sshpool.ClientForCredential(&host, cred)
			if err != nil {
				updateItem(item.ID, "stop", "failed", "连接失败: "+err.Error())
				return
			}
			defer cli.Close()

			backupDir := ah.BackupDir
			if backupDir == "" {
				backupDir = path.Join(ah.DeployDir, "backup")
			}
			stopCmd := ah.StopCmd
			if stopCmd == "" {
				stopCmd = fmt.Sprintf("pkill -f '%s' || true", ah.JarName)
			}
			rollbackCmd := fmt.Sprintf(
				"latest=$(ls -t '%s'/'%s'.*.bak 2>/dev/null | head -1); "+
					"if [ -z \"$latest\" ]; then echo NO_BACKUP; exit 1; fi; "+
					"cp -a \"$latest\" '%s'/'%s'; echo RESTORED: $latest",
				backupDir, ah.JarName, ah.DeployDir, ah.JarName)
			startCmd := ah.StartCmd
			if startCmd == "" {
				startCmd = fmt.Sprintf("cd '%s' && nohup java -jar '%s' > /dev/null 2>&1 &", ah.DeployDir, ah.JarName)
			}
			fullCmd := stopCmd + "; " + rollbackCmd + "; " + startCmd
			var buf strings.Builder
			code, err := sshpool.RunCommand(context.Background(), cli, fullCmd, 10*time.Minute, func(s string) { buf.WriteString(s) })
			out := buf.String()
			status := "success"
			step := "start"
			if err != nil || code != 0 || strings.Contains(out, "NO_BACKUP") {
				status = "failed"
				step = "backup"
				if !strings.Contains(out, "NO_BACKUP") {
					step = "start"
				}
			}
			updateItem(item.ID, step, status, out)
		}(item)
	}
	wg.Wait()
	var fails int64
	model.DB.Model(&model.ReleaseItem{}).Where("release_id = ? AND status = ?", rel.ID, "failed").Count(&fails)
	st := "success"
	if fails > 0 {
		st = "failed"
	}
	model.DB.Model(&model.Release{}).Where("id = ?", rel.ID).Update("status", st)
	ws.H.Broadcast(fmt.Sprintf("release-%d", rel.ID), map[string]any{"type": "release_done", "status": st})
}

// resolveReleaseCredential picks the OS account for a release: the AppHost-specified one first, otherwise the host default credential
func resolveReleaseCredential(ah *model.AppHost, host *model.Host) (*model.HostCredential, error) {
	if ah.CredentialID != nil {
		var cred model.HostCredential
		if err := model.DB.First(&cred, *ah.CredentialID).Error; err != nil {
			return nil, fmt.Errorf("指定的 OS 账号不存在")
		}
		return &cred, nil
	}
	var cred model.HostCredential
	if err := model.DB.Where("host_id = ?", host.ID).Order("is_default DESC, id ASC").First(&cred).Error; err != nil {
		return nil, fmt.Errorf("主机未配置 OS 账号")
	}
	return &cred, nil
}

func updateItem(itemID uint, step, status, log string) {
	model.DB.Model(&model.ReleaseItem{}).Where("id = ?", itemID).
		Updates(map[string]any{"step": step, "status": status, "log": log})
	var item model.ReleaseItem
	if err := model.DB.First(&item, itemID).Error; err == nil {
		ws.H.Broadcast(fmt.Sprintf("release-%d", item.ReleaseID), map[string]any{
			"type": "item", "item": item,
		})
	}
}

func healthCheck(url string, retries int, interval time.Duration) bool {
	for i := 0; i < retries; i++ {
		resp, err := httpTimeoutClient.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return true
			}
		}
		time.Sleep(interval)
	}
	return false
}

// UserCanDeployApp reports whether the user can deploy the given app (admin / authorized ops or releaser)
func UserCanDeployApp(user *model.User, appID uint) bool {
	if PlatformRole(user.Role) {
		return true
	}
	// Custom role: having the releases capability bit (already validated at the routing layer) counts as an authorized deployer;
	// data-level access is determined by user-group app bindings (see CanOperateApp)
	// Personal authorization (legacy mechanism)
	var cnt int64
	model.DB.Model(&model.UserApp{}).Where("user_id = ? AND app_id = ?", user.ID, appID).Count(&cnt)
	if cnt > 0 {
		return true
	}
	// User-group app binding (team authorization)
	return CanOperateApp(user, appID)
}
