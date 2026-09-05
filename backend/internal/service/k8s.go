// AutoOps 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"autoops/internal/model"
	"autoops/internal/pkg"
)

// K8S 集群集成：通过 API Server + 证书认证纳管集群，
// 探测在线状态/版本/节点数，记录并监控证书有效期。

type k8sKubeconfig struct {
	CurrentContext string `yaml:"current-context"`
	Contexts       []struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster string `yaml:"cluster"`
			User    string `yaml:"user"`
		} `yaml:"context"`
	} `yaml:"contexts"`
	Clusters []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			CertificateAuthorityData string `yaml:"certificate-authority-data"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
	Users []struct {
		Name string `yaml:"name"`
		User struct {
			ClientCertificateData string `yaml:"client-certificate-data"`
			ClientKeyData         string `yaml:"client-key-data"`
		} `yaml:"user"`
	} `yaml:"users"`
}

// ParseKubeconfig 解析 kubeconfig，返回 server / CA / 客户端证书 / 私钥（PEM 明文）
func ParseKubeconfig(raw string) (server, ca, cert, key string, err error) {
	var kc k8sKubeconfig
	if err = yaml.Unmarshal([]byte(raw), &kc); err != nil {
		return "", "", "", "", fmt.Errorf("kubeconfig 格式错误: %w", err)
	}
	ctxName := kc.CurrentContext
	var ctxCluster, ctxUser string
	for _, c := range kc.Contexts {
		if c.Name == ctxName {
			ctxCluster, ctxUser = c.Context.Cluster, c.Context.User
		}
	}
	if ctxCluster == "" && len(kc.Clusters) > 0 {
		ctxCluster = kc.Clusters[0].Name
	}
	if ctxUser == "" && len(kc.Users) > 0 {
		ctxUser = kc.Users[0].Name
	}
	for _, c := range kc.Clusters {
		if c.Name == ctxCluster {
			server = c.Cluster.Server
			ca = c.Cluster.CertificateAuthorityData
		}
	}
	for _, u := range kc.Users {
		if u.Name == ctxUser {
			cert = u.User.ClientCertificateData
			key = u.User.ClientKeyData
		}
	}
	if server == "" {
		return "", "", "", "", fmt.Errorf("kubeconfig 中未找到 API Server")
	}
	return server, ca, cert, key, nil
}

// k8sTLSClient 用集群凭据构造带客户端证书的 HTTP 客户端
func k8sTLSClient(caPEM, certPEM, keyPEM string, timeout time.Duration) (*http.Client, error) {
	caPool := x509.NewCertPool()
	if caPEM != "" && !caPool.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, fmt.Errorf("CA 证书解析失败")
	}
	tlsCfg := &tls.Config{RootCAs: caPool, MinVersion: tls.VersionTLS12}
	if certPEM != "" && keyPEM != "" {
		pair, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			return nil, fmt.Errorf("客户端证书/私钥不匹配: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{pair}
	}
	return &http.Client{Timeout: timeout, Transport: &http.Transport{TLSClientConfig: tlsCfg}}, nil
}

// K8sProbeResult 探测结果
type K8sProbeResult struct {
	Version   string
	NodeCount int
	CertExp   *time.Time
	CAExp     *time.Time
}

// ProbeK8sCluster 连接集群：取版本 / 节点数 / 证书到期
func ProbeK8sCluster(c *model.K8sCluster) (*K8sProbeResult, error) {
	caPEM, certPEM, keyPEM := decryptK8sCreds(c)
	cli, err := k8sTLSClient(caPEM, certPEM, keyPEM, 10*time.Second)
	if err != nil {
		return nil, err
	}
	res := &K8sProbeResult{}

	// /version（无需鉴权，用于连通性与版本）
	resp, err := cli.Get(stringsTrimRightSlash(c.ApiServer) + "/version")
	if err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	var ver struct {
		GitVersion string `json:"gitVersion"`
		Major      string `json:"major"`
		Minor      string `json:"minor"`
	}
	if err := json.Unmarshal(body, &ver); err == nil && ver.GitVersion != "" {
		res.Version = ver.GitVersion
	}

	// /api/v1/nodes（需要客户端证书）
	resp2, err := cli.Get(stringsTrimRightSlash(c.ApiServer) + "/api/v1/nodes")
	if err == nil {
		b2, _ := io.ReadAll(io.LimitReader(resp2.Body, 5<<20))
		resp2.Body.Close()
		var nodes struct {
			Items []struct {
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
			} `json:"items"`
		}
		if json.Unmarshal(b2, &nodes) == nil {
			res.NodeCount = len(nodes.Items)
		}
	}

	// 证书有效期（客户端证书优先，其次 CA）
	if certPEM != "" {
		if exp := pemNotAfter(certPEM); exp != nil {
			res.CertExp = exp
		}
	}
	if caPEM != "" {
		if exp := pemNotAfter(caPEM); exp != nil {
			res.CAExp = exp
		}
	}
	return res, nil
}

func firstPEMBlock(pemData string) *pem.Block {
	for {
		block, rest := pem.Decode([]byte(pemData))
		if block == nil {
			return nil
		}
		if block.Type == "CERTIFICATE" {
			return block
		}
		pemData = string(rest)
	}
}

func pemNotAfter(pemData string) *time.Time {
	// 仅取第一块证书
	block := firstPEMBlock(pemData)
	if block == nil {
		return nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil
	}
	return &cert.NotAfter
}

func stringsTrimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// ---------- 凭据加解密 ----------

func encryptK8sSecret(plain string) string {
	if plain == "" {
		return ""
	}
	enc, err := pkg.Encrypt(plain)
	if err != nil {
		return ""
	}
	return enc
}

// EncryptK8sSecret AES-GCM 加密集群凭据
func EncryptK8sSecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	return pkg.Encrypt(plain)
}

func decryptK8sCreds(c *model.K8sCluster) (ca, cert, key string) {
	// kubeconfig 模式：从加密的 kubeconfig 中解析三件套
	if c.Kubeconfig != "" {
		if raw, err := pkg.Decrypt(c.Kubeconfig); err == nil {
			server, ca, cert, key, err := ParseKubeconfig(raw)
			if err == nil && c.ApiServer == "" {
				c.ApiServer = server
			}
			if err == nil {
				return ca, cert, key
			}
		}
	}
	ca, _ = pkg.Decrypt(c.CA)
	cert, _ = pkg.Decrypt(c.ClientCert)
	key, _ = pkg.Decrypt(c.ClientKey)
	return ca, cert, key
}

// ---------- 周期探测与证书到期提醒 ----------

var k8sProbeMu sync.Mutex
var k8sLastProbe = map[uint]time.Time{}

// k8sLastRemind 证书提醒冷却（每集群每级别 20h）
var remindMu sync.Mutex
var k8sLastRemind = map[string]time.Time{}

// CollectK8sClusters 探测全部集群状态/版本/节点（5 分钟节流）
func CollectK8sClusters() {
	var clusters []model.K8sCluster
	model.DB.Where("enabled = ?", true).Find(&clusters)
	now := time.Now()
	k8sProbeMu.Lock()
	var due []model.K8sCluster
	for _, c := range clusters {
		if last, ok := k8sLastProbe[c.ID]; !ok || now.Sub(last) >= 5*time.Minute {
			due = append(due, c)
			k8sLastProbe[c.ID] = now
		}
	}
	k8sProbeMu.Unlock()
	for _, c := range due {
		go func(c model.K8sCluster) {
			defer func() { recover() }()
			res, err := ProbeK8sCluster(&c)
			updates := map[string]any{"last_seen": now}
			if err != nil {
				updates["status"] = "offline"
			} else {
				updates["status"] = "online"
				if res.Version != "" {
					updates["version"] = res.Version
				}
				if res.NodeCount > 0 {
					updates["node_count"] = res.NodeCount
				}
				if res.CertExp != nil {
					if c.CertExpiry == nil || !c.CertExpiry.Equal(*res.CertExp) {
						updates["warn30_sent"] = false
						updates["warn7_sent"] = false
					}
					updates["cert_expiry"] = res.CertExp
				}
				if res.CAExp != nil {
					updates["ca_expiry"] = res.CAExp
				}
			}
			model.DB.Model(&model.K8sCluster{}).Where("id = ?", c.ID).Updates(updates)
			CheckK8sCertExpiry(&c, res)
		}(c)
	}
}

// CheckK8sCertExpiry 证书到期提醒：剩余 30 天 / 7 天各提醒一次（恢复证书后重置）
func CheckK8sCertExpiry(c *model.K8sCluster, res *K8sProbeResult) {
	if res == nil || res.CertExp == nil {
		return
	}
	days := int(time.Until(*res.CertExp).Hours() / 24)
	var channels []model.AlertChannel
	model.DB.Where("enabled = ?", true).Find(&channels)
	if len(channels) == 0 {
		return
	}
	send := func(title, text string) {
		for _, ch := range channels {
			ch := ch
			go func() {
				defer func() { recover() }()
				if err := SendViaChannel(&ch, map[string]string{"time": time.Now().Format("2006-01-02 15:04:05")}, title, text); err != nil {
					fmt.Printf("[k8s] channel %s send failed: %v\n", ch.Name, err)
				}
			}()
		}
	}
	text := fmt.Sprintf("集群: %s\nAPI Server: %s\n支持人: %s\n证书到期: %s\n剩余约 %d 天",
		c.Name, c.ApiServer, c.Support, res.CertExp.Format("2006-01-02"), days)
	// 提醒冷却：每集群每级别 20 小时内不重复推送（防止探测频率变化导致的重复告警）
	remind := func(lv string) bool {
		key := fmt.Sprintf("%d:%s", c.ID, lv)
		remindMu.Lock()
		defer remindMu.Unlock()
		if last, ok := k8sLastRemind[key]; ok && time.Since(last) < 20*time.Hour {
			return false
		}
		k8sLastRemind[key] = time.Now()
		return true
	}
	if days <= 7 && !c.Warn7Sent && remind("7") {
		send("🔴 [证书即将到期] "+c.Name, text+"\n级别: P1（剩余不足 7 天）")
		model.DB.Model(c).Update("warn7_sent", true)
	} else if days <= 30 && !c.Warn30Sent && !c.Warn7Sent && remind("30") {
		send("🟠 [证书即将到期] "+c.Name, text+"\n级别: P2（剩余不足 30 天）")
		model.DB.Model(c).Updates(map[string]any{"warn30_sent": true, "warn7_sent": true})
	}
}

var _ = sync.Once{}
