// JNexus 运维平台 — By JJ Zhang, Version 1.0

package sshpool

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"autoops/internal/model"
	"autoops/internal/pkg"
)

// ClientFor 根据主机记录建立 SSH 连接（使用主机默认凭据；无凭据时回退主机自带账号）
func ClientFor(h *model.Host) (*gossh.Client, error) {
	var cred model.HostCredential
	if err := model.DB.Where("host_id = ?", h.ID).Order("is_default DESC, id ASC").First(&cred).Error; err == nil {
		return ClientForCredential(h, &cred)
	}
	return connect(h, h.Username, h.AuthType, h.SSHKeyID, h.Password)
}

// ClientForCredential 用指定的 OS 账号（凭据）连接主机
func ClientForCredential(h *model.Host, cred *model.HostCredential) (*gossh.Client, error) {
	return connect(h, cred.Username, cred.AuthType, cred.SSHKeyID, cred.Password)
}

func connect(h *model.Host, username, authType string, sshKeyID *uint, encPassword string) (*gossh.Client, error) {
	var auth gossh.AuthMethod
	switch authType {
	case "password":
		pwd, err := pkg.Decrypt(encPassword)
		if err != nil {
			return nil, fmt.Errorf("解密密码失败: %w", err)
		}
		auth = gossh.Password(pwd)
	default: // key
		if sshKeyID == nil {
			return nil, fmt.Errorf("主机未配置认证方式")
		}
		var key model.SSHKey
		if err := model.DB.First(&key, *sshKeyID).Error; err != nil {
			return nil, fmt.Errorf("密钥不存在: %w", err)
		}
		privPEM, err := pkg.Decrypt(key.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("解密私钥失败: %w", err)
		}
		signer, err := gossh.ParsePrivateKey([]byte(privPEM))
		if err != nil {
			return nil, fmt.Errorf("解析私钥失败: %w", err)
		}
		auth = gossh.PublicKeys(signer)
	}
	port := h.Port
	if port == 0 {
		port = 22
	}
	cfg := &gossh.ClientConfig{
		User:            username,
		Auth:            []gossh.AuthMethod{auth},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // 内网运维平台，忽略主机指纹校验
		Timeout:         10 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", h.IP, port)
	cli, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("连接 %s 失败: %w", addr, err)
	}
	return cli, nil
}

// OutputWriter 命令输出回调
type OutputWriter func(chunk string)

// RunCommand 在主机上执行命令，实时回调输出；返回 exit code
func RunCommand(ctx context.Context, cli *gossh.Client, cmd string, timeout time.Duration, onOut OutputWriter) (int, error) {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sess, err := cli.NewSession()
	if err != nil {
		return -1, err
	}
	defer sess.Close()

	stdout, err := sess.StdoutPipe()
	if err != nil {
		return -1, err
	}
	stderr, err := sess.StderrPipe()
	if err != nil {
		return -1, err
	}

	if err := sess.Start(cmd); err != nil {
		return -1, err
	}

	done := make(chan struct{})
	go func() {
		copyStream(onOut, stdout)
		copyStream(onOut, stderr)
		close(done)
	}()

	exitCode := 0
	errCh := make(chan error, 1)
	go func() { errCh <- sess.Wait() }()

	select {
	case <-ctx.Done():
		_ = sess.Signal(gossh.SIGKILL)
		return -1, fmt.Errorf("执行超时(%s)", timeout)
	case err := <-errCh:
		<-done
		if err != nil {
			if ee, ok := err.(*gossh.ExitError); ok {
				exitCode = ee.ExitStatus()
			} else {
				return -1, err
			}
		}
		return exitCode, nil
	}
}

func copyStream(onOut OutputWriter, r io.Reader) {
	if r == nil || onOut == nil {
		io.Copy(io.Discard, r)
		return
	}
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			onOut(string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

// Probe TCP 探测端口
func Probe(ip string, port int, timeout time.Duration) error {
	if port == 0 {
		port = 22
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), timeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// ParsePrivateKeyFile 读取私钥文件并返回 PEM 内容与公钥字符串
func ParsePrivateKeyFile(path string) (privPEM, pubSSH string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	privPEM = string(data)
	signer, err := gossh.ParsePrivateKey(data)
	if err != nil {
		// 可能是加密私钥，返回 PEM 让用户自行处理
		return privPEM, "", nil
	}
	pubSSH = strings.TrimSpace(string(gossh.MarshalAuthorizedKey(signer.PublicKey())))
	return privPEM, pubSSH, nil
}

// B64 帮助函数（保留给扩展使用）
func B64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
