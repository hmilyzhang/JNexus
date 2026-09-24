// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// Trusted CA store: admins upload internal-company or public root/intermediate
// certificates in the UI; every outbound TLS connection the platform makes
// (app monitors, OpenObserve, alert webhooks, ticketing, AI, SMTP STARTTLS,
// cloud integrations) trusts the system pool PLUS these uploads. Adding or
// removing a certificate takes effect immediately — the merged pool is rebuilt
// on demand, no restart needed.

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"
	"sync"
	"time"

	"jnexus/internal/model"
)

const (
	maxTrustedCAEntries = 50
	maxTrustedCAPEMSize = 256 * 1024
)

// TrustedCAList returns all uploaded CA certificates (PEM body masked out)
func TrustedCAList() []model.TrustedCA {
	var out []model.TrustedCA
	model.DB.Order("id").Find(&out)
	for i := range out {
		out[i].PEM = ""
	}
	return out
}

// AddTrustedCA validates and stores a PEM (may contain a chain: root +
// intermediates). Deduplicated by SHA-256 fingerprint of the whole body.
func AddTrustedCA(name, pemText, operator string) (*model.TrustedCA, error) {
	pemText = strings.TrimSpace(pemText)
	if pemText == "" {
		return nil, fmt.Errorf("证书内容为空")
	}
	if len(pemText) > maxTrustedCAPEMSize {
		return nil, fmt.Errorf("证书内容过大（上限 256KB）")
	}
	if !strings.Contains(pemText, "-----BEGIN CERTIFICATE-----") {
		return nil, fmt.Errorf("不是 PEM 证书（缺少 BEGIN CERTIFICATE）")
	}
	rest := []byte(pemText)
	count := 0
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("包含非证书 PEM 块（%s）", block.Type)
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return nil, fmt.Errorf("证书解析失败: %w", err)
		}
		count++
	}
	if count == 0 {
		return nil, fmt.Errorf("证书解析失败：未找到证书")
	}

	var cnt int64
	model.DB.Model(&model.TrustedCA{}).Count(&cnt)
	if cnt >= maxTrustedCAEntries {
		return nil, fmt.Errorf("最多保存 %d 条证书", maxTrustedCAEntries)
	}

	sum := sha256.Sum256([]byte(pemText))
	fp := hex.EncodeToString(sum[:])
	var dup model.TrustedCA
	if err := model.DB.Where("fingerprint = ?", fp).First(&dup).Error; err == nil {
		return nil, fmt.Errorf("该证书已存在（%s）", dup.Name)
	}

	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("ca-%s", time.Now().Format("0102150405"))
	}
	rec := &model.TrustedCA{
		Name: name, PEM: pemText, Fingerprint: fp,
		CreatedBy: operator, CreatedAt: time.Now(),
	}
	if err := model.DB.Create(rec).Error; err != nil {
		return nil, err
	}
	invalidateCAPool()
	return rec, nil
}

// DeleteTrustedCA removes an uploaded certificate by id
func DeleteTrustedCA(id uint) error {
	var rec model.TrustedCA
	if err := model.DB.First(&rec, id).Error; err != nil {
		return fmt.Errorf("证书不存在")
	}
	if err := model.DB.Delete(&rec).Error; err != nil {
		return err
	}
	invalidateCAPool()
	return nil
}

// ---------- merged outbound pool ----------

var (
	caPoolMu       sync.Mutex
	caPoolGen      int            // bumped on every add/delete
	caPoolLoaded   = -1           // generation the cache was built from
	caPoolCache    *x509.CertPool // valid only when caPoolHasEntries
	caPoolHasEntry bool
)

func invalidateCAPool() {
	caPoolMu.Lock()
	caPoolGen++
	caPoolMu.Unlock()
}

// TrustedCAPool returns the RootCAs for outbound TLS connections:
// the system pool (honours SSL_CERT_FILE) plus every uploaded certificate.
// Returns nil when nothing was uploaded — callers then keep the default
// behavior (RootCAs nil = system pool).
func TrustedCAPool() *x509.CertPool {
	caPoolMu.Lock()
	defer caPoolMu.Unlock()
	if caPoolLoaded == caPoolGen {
		if caPoolHasEntry {
			return caPoolCache
		}
		return nil
	}
	var rows []model.TrustedCA
	model.DB.Select("id, name, pem").Find(&rows)
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	added := 0
	for _, r := range rows {
		rest := []byte(r.PEM)
		for {
			var block *pem.Block
			block, rest = pem.Decode(rest)
			if block == nil {
				break
			}
			if cert, perr := x509.ParseCertificate(block.Bytes); perr == nil {
				pool.AddCert(cert)
				added++
			}
		}
	}
	caPoolLoaded = caPoolGen
	caPoolHasEntry = added > 0
	caPoolCache = pool
	if !caPoolHasEntry {
		return nil
	}
	return pool
}

// OutboundTLS returns a TLS config trusting system + uploaded CAs,
// or nil when no uploads exist (callers keep their default TLS config).
func OutboundTLS() *tls.Config {
	pool := TrustedCAPool()
	if pool == nil {
		return nil
	}
	return &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
}
