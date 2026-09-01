// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"autoops/internal/model"
	"autoops/internal/pkg"
	"autoops/internal/ws"
)

// BatchCredRequest 批量为存量主机添加 OS 账号
type BatchCredRequest struct {
	HostIDs     []uint `json:"host_ids"`
	GroupID     *uint  `json:"group_id"`
	IPs         []string `json:"ips"`
	Username    string `json:"username" binding:"required"`
	Label       string `json:"label"`
	AuthType    string `json:"auth_type"` // key / password
	SSHKeyID    *uint  `json:"ssh_key_id"`
	Password    string `json:"password"` // 统一密码（自动配对或密码认证）
	AutoPair    bool   `json:"auto_pair"`
	Concurrency int    `json:"concurrency"`
}

// BatchAddCredentials 批量添加：创建任务后异步并发执行，进度经 WS 与任务记录可见
func BatchAddCredentials(operator *model.User, req BatchCredRequest) (uint, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return 0, fmt.Errorf("账号名不能为空")
	}
	authType := req.AuthType
	if authType == "" {
		authType = "key"
	}
	if authType == "key" && req.SSHKeyID == nil && !(req.AutoPair && req.Password != "") {
		return 0, fmt.Errorf("密钥认证需要选择 SSH 密钥，或使用密码自动配对")
	}
	if authType == "password" && req.Password == "" {
		return 0, fmt.Errorf("密码认证需要填写统一密码")
	}
	hosts, err := resolveHosts(operator, req.HostIDs, req.GroupID, req.IPs)
	if err != nil {
		return 0, err
	}
	if len(hosts) == 0 {
		return 0, fmt.Errorf("未选择任何有权限的目标主机")
	}

	conc := req.Concurrency
	if conc <= 0 {
		conc = 20
	}

	params, _ := json.Marshal(map[string]any{
		"username": username, "label": req.Label, "auth_type": authType,
		"auto_pair": req.AutoPair, "hosts": len(hosts),
	})
	task := model.Task{Type: model.TaskCred, Operator: operator.Username, Params: string(params), Status: "running"}
	if err := model.DB.Create(&task).Error; err != nil {
		return 0, err
	}
	results := make([]model.TaskHostResult, 0, len(hosts))
	for _, h := range hosts {
		results = append(results, model.TaskHostResult{
			TaskID: task.ID, HostID: h.ID, HostIP: h.IP, HostName: h.Name, Status: "pending",
		})
	}
	if err := model.DB.Create(&results).Error; err != nil {
		return 0, err
	}
	go runBatchCred(operator, req, username, authType, task.ID, hosts, results, conc)
	return task.ID, nil
}

func runBatchCred(operator *model.User, req BatchCredRequest, username, authType string, taskID uint, hosts []model.Host, results []model.TaskHostResult, conc int) {

	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res := results[idx]
			host := hosts[idx]

			// 已存在同名账号则跳过
			var cnt int64
			model.DB.Model(&model.HostCredential{}).
				Where("host_id = ? AND username = ?", host.ID, username).Count(&cnt)
			if cnt > 0 {
				finishCredResult(taskID, res.ID, 0, "已存在同名账号，跳过")
				return
			}

			// 自动配对：密码登录推送平台公钥，成功仅登记密钥凭据
			if req.AutoPair && req.Password != "" {
				paired, _, perr := PairAndCreateCredential(&host, username, req.Password, req.Label, false)
				if perr != nil {
					finishCredResult(taskID, res.ID, -1, "配对失败: "+perr.Error())
					return
				}
				_ = paired
				finishCredResult(taskID, res.ID, 0, "已添加并完成密钥配对")
				return
			}

			useAuthType := authType
			encPwd := ""
			sshKeyID := req.SSHKeyID
			if useAuthType == "password" {
				enc, err := pkg.Encrypt(req.Password)
				if err != nil {
					finishCredResult(taskID, res.ID, -1, err.Error())
					return
				}
				encPwd = enc
			}

			var haveCred int64
			model.DB.Model(&model.HostCredential{}).Where("host_id = ?", host.ID).Count(&haveCred)
			cred := model.HostCredential{
				HostID: host.ID, Username: username, AuthType: useAuthType,
				SSHKeyID: sshKeyID, Password: encPwd, Label: req.Label, IsDefault: haveCred == 0,
			}
			if err := model.DB.Create(&cred).Error; err != nil {
				finishCredResult(taskID, res.ID, -1, "创建凭据失败: "+err.Error())
				return
			}
			finishCredResult(taskID, res.ID, 0, "已添加")
		}(i)
	}
	wg.Wait()
	finalizeCredTask(taskID)
}

func dialWithPassword(host model.Host, username, password string) (*gossh.Client, error) {
	port := host.Port
	if port == 0 {
		port = 22
	}
	cfg := &gossh.ClientConfig{
		User:            username,
		Auth:            []gossh.AuthMethod{gossh.Password(password)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	cli, err := gossh.Dial("tcp", fmt.Sprintf("%s:%d", host.IP, port), cfg)
	if err != nil {
		return nil, err
	}
	return cli, nil
}

func installPubKey(cli *gossh.Client, pubLine string) (int, error) {
	cmd := fmt.Sprintf(
		"mkdir -p ~/.ssh && chmod 700 ~/.ssh && touch ~/.ssh/authorized_keys && "+
			"grep -qxF '%s' ~/.ssh/authorized_keys || echo '%s' >> ~/.ssh/authorized_keys; "+
			"chmod 600 ~/.ssh/authorized_keys", pubLine, pubLine)
	return RunCommandBackground(cli, cmd)
}

func finishCredResult(taskID, resID uint, code int, msg string) {
	now := time.Now()
	model.DB.Model(&model.TaskHostResult{}).Where("id = ?", resID).
		Updates(map[string]any{"exit_code": code, "output": msg, "finished_at": now})
	var res model.TaskHostResult
	if err := model.DB.First(&res, resID).Error; err == nil {
		ws.H.Broadcast(taskTopic(taskID), map[string]any{
			"type": "cred_result", "result": res,
		})
	}
}

func finalizeCredTask(taskID uint) {
	var fails int64
	model.DB.Model(&model.TaskHostResult{}).Where("task_id = ? AND status = ?", taskID, "failed").Count(&fails)
	status := "done"
	if fails > 0 {
		status = "failed"
	}
	model.DB.Model(&model.Task{}).Where("id = ?", taskID).
		Updates(map[string]any{"status": status, "finished_at": time.Now()})
	ws.H.Broadcast(taskTopic(taskID), map[string]any{"type": "task_done", "status": status})
	NotifyTaskFinished(taskID)
}
