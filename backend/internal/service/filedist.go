// AutoOps 运维平台 — By JJ Zhang, Version 1.0

package service

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"

	"autoops/internal/model"
	"autoops/internal/sshpool"
	"autoops/internal/ws"
)

// DistributeRequest 批量分发请求
type DistributeRequest struct {
	HostIDs    []uint   `json:"host_ids"`
	GroupID    *uint    `json:"group_id"`
	IPs        []string `json:"ips"` // 多 IP 逗号分隔输入
	RemoteDir  string   `json:"remote_dir"`
	RemoteName string   `json:"remote_name"` // 可选，重命名
	LocalFile  string   `json:"local_file"`  // 已上传到服务端的文件名
}

// DistributeFile 把已上传的本地文件并发 SFTP 分发到目标主机
func DistributeFile(operator *model.User, localPath, localName string, req DistributeRequest) (uint, []string, error) {
	if req.RemoteDir == "" {
		return 0, nil, fmt.Errorf("目标目录不能为空")
	}
	hosts, err := resolveHosts(operator, req.HostIDs, req.GroupID, req.IPs)
	if err != nil {
		return 0, nil, err
	}
	if len(hosts) == 0 {
		return 0, nil, fmt.Errorf("未选择任何有权限的目标主机")
	}

	params, _ := json.Marshal(map[string]any{
		"file": localName, "remote_dir": req.RemoteDir, "hosts": len(hosts),
	})
	task := model.Task{Type: model.TaskFile, Operator: operator.Username, Params: string(params), Status: "running"}
	if err := model.DB.Create(&task).Error; err != nil {
		return 0, nil, err
	}

	results := make([]model.TaskHostResult, 0, len(hosts))
	for _, h := range hosts {
		results = append(results, model.TaskHostResult{
			TaskID: task.ID, HostID: h.ID, HostIP: h.IP, HostName: h.Name, Status: "pending",
		})
	}
	if err := model.DB.Create(&results).Error; err != nil {
		return 0, nil, err
	}

	go runDistribute(task.ID, localPath, localName, req.RemoteDir, req.RemoteName, results)
	return task.ID, nil, nil
}

func runDistribute(taskID uint, localPath, localName, remoteDir, remoteName string, results []model.TaskHostResult) {
	fi, err := os.Stat(localPath)
	total := int64(0)
	if err == nil {
		total = fi.Size()
	}
	name := remoteName
	if name == "" {
		name = localName
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	remain := len(results)

	finish := func(resID uint, code int, msg, status string) {
		now := time.Now()
		model.DB.Model(&model.TaskHostResult{}).Where("id = ?", resID).
			Updates(map[string]any{"exit_code": code, "output": msg, "status": status, "finished_at": now})
		ws.H.Broadcast(taskTopic(taskID), map[string]any{"type": "status", "result_id": resID, "status": status})
		mu.Lock()
		remain--
		left := remain
		mu.Unlock()
		if left == 0 {
			st := "done"
			var fails int64
			model.DB.Model(&model.TaskHostResult{}).Where("task_id = ? AND status = ?", taskID, "failed").Count(&fails)
			if fails > 0 {
				st = "failed"
			}
			model.DB.Model(&model.Task{}).Where("id = ?", taskID).
				Updates(map[string]any{"status": st, "finished_at": now})
			ws.H.Broadcast(taskTopic(taskID), map[string]any{"type": "task_done", "status": st})
		}
	}

	for _, res := range results {
		wg.Add(1)
		go func(res model.TaskHostResult) {
			defer wg.Done()
			host := model.Host{}
			model.DB.First(&host, res.HostID)

			cli, err := sshpool.ClientFor(&host)
			if err != nil {
				finish(res.ID, -1, "连接失败: "+err.Error(), "failed")
				return
			}
			defer cli.Close()

			msg, code := sftpUpload(cli, localPath, remoteDir, name, total, taskID, res.ID)
			status := "success"
			if code != 0 {
				status = "failed"
			}
			finish(res.ID, code, msg, status)
		}(res)
	}
	wg.Wait()
}

// sftpUpload 上传文件（自动创建远程目录），返回日志与退出码
func sftpUpload(cli *gossh.Client, localPath, remoteDir, remoteName string, total int64, taskID, resID uint) (string, int) {
	scli, err := sftp.NewClient(cli)
	if err != nil {
		return "SFTP 建立失败: " + err.Error(), -1
	}
	defer scli.Close()

	if err := mkdirAll(scli, remoteDir); err != nil {
		return "创建远程目录失败: " + err.Error(), -1
	}
	remotePath := remoteDir + "/" + remoteName

	local, err := os.Open(localPath)
	if err != nil {
		return "打开本地文件失败: " + err.Error(), -1
	}
	defer local.Close()

	remote, err := scli.Create(remotePath)
	if err != nil {
		return "创建远程文件失败: " + err.Error(), -1
	}
	defer remote.Close()

	written := int64(0)
	buf := make([]byte, 128*1024)
	lastPct := -1
	for {
		n, rerr := local.Read(buf)
		if n > 0 {
			if _, werr := remote.Write(buf[:n]); werr != nil {
				return "写入远程文件失败: " + werr.Error(), -1
			}
			written += int64(n)
			if total > 0 {
				pct := int(written * 100 / total)
				if pct/10 > lastPct {
					lastPct = pct / 10
					ws.H.Broadcast(taskTopic(taskID), map[string]any{
						"type": "progress", "result_id": resID, "percent": pct,
					})
				}
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return "读取本地文件失败: " + rerr.Error(), -1
		}
	}
	// 保持可执行权限（针对脚本/二进制）
	mode := os.FileMode(0644)
	if filepath.Ext(remoteName) == ".sh" {
		mode = 0755
	}
	_ = scli.Chmod(remotePath, mode)
	return fmt.Sprintf("已上传 %s -> %s:%s (%d bytes)", localPath, remoteDir, remoteName, written), 0
}

// mkdirAll 递归创建远程目录
func mkdirAll(scli *sftp.Client, dir string) error {
	if dir == "" || dir == "/" || dir == "." {
		return nil
	}
	if _, err := scli.Stat(dir); err == nil {
		return nil
	}
	parent := filepath.ToSlash(filepath.Dir(dir))
	if parent != dir {
		if err := mkdirAll(scli, parent); err != nil {
			return err
		}
	}
	return scli.Mkdir(dir)
}

// DownloadFromHost 从主机下载文件到本地临时目录，返回本地路径
func DownloadFromHost(host *model.Host, remotePath string) (string, error) {
	cli, err := sshpool.ClientFor(host)
	if err != nil {
		return "", err
	}
	defer cli.Close()
	scli, err := sftp.NewClient(cli)
	if err != nil {
		return "", err
	}
	defer scli.Close()

	src, err := scli.Open(remotePath)
	if err != nil {
		return "", fmt.Errorf("打开远程文件失败: %w", err)
	}
	defer src.Close()

	localPath := filepath.Join(os.TempDir(), fmt.Sprintf("autoops-dl-%d-%s", time.Now().UnixNano(), filepath.Base(remotePath)))
	dst, err := os.Create(localPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}
	return localPath, nil
}
