// JNexus 运维平台 — By JJ Zhang, Version 1.0

package service

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	goldap "github.com/go-ldap/ldap/v3"

	"jnexus/internal/model"
)

const ldapTimeout = 10 * time.Second

// SystemConfigMap 读取全部系统配置为 map
func SystemConfigMap() map[string]string {
	out := map[string]string{}
	var rows []model.SystemConfig
	model.DB.Find(&rows)
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out
}

// SetSystemConfigs 批量写入配置（key 不存在则创建）
func SetSystemConfigs(m map[string]string) error {
	for k, v := range m {
		if err := model.DB.Save(&model.SystemConfig{Key: k, Value: v}).Error; err != nil {
			return err
		}
	}
	return nil
}

// LDAPSettings 从系统配置提取 LDAP 设置
type LDAPSettings struct {
	Enabled        bool
	Host           string
	Port           int
	TLS            bool
	BindDN         string
	BindPassword   string
	BaseDN         string
	UserFilter     string
	AttrUsername   string
	DefaultRole    string
	GroupCheck     bool     // 启用用户组校验
	GroupBaseDN    string   // 用户组搜索 Base DN
	GroupFilter    string   // 组过滤器，%s 替换为用户 DN
	RequiredGroups []string // 允许登录的用户组 DN/CN 列表
}

func LoadLDAPSettings() LDAPSettings {
	m := SystemConfigMap()
	s := LDAPSettings{
		Enabled:      m["ldap_enabled"] == "true",
		Host:         m["ldap_host"],
		TLS:          m["ldap_tls"] == "true",
		BindDN:       m["ldap_bind_dn"],
		BindPassword: m["ldap_bind_password"],
		BaseDN:       m["ldap_base_dn"],
		UserFilter:   m["ldap_user_filter"],
		AttrUsername: m["ldap_attr_username"],
		DefaultRole:  m["ldap_default_role"],
	}
	if s.UserFilter == "" {
		s.UserFilter = "(uid=%s)"
	}
	if s.AttrUsername == "" {
		s.AttrUsername = "uid"
	}
	if s.DefaultRole == "" {
		s.DefaultRole = model.RoleViewer
	}
	fmt.Sscanf(m["ldap_port"], "%d", &s.Port)
	if s.Port == 0 {
		s.Port = 389
	}
	s.GroupCheck = m["ldap_group_check"] == "true"
	s.GroupBaseDN = m["ldap_group_base_dn"]
	s.GroupFilter = m["ldap_group_filter"]
	if s.GroupFilter == "" {
		s.GroupFilter = "(member=%s)"
	}
	for _, g := range strings.FieldsFunc(m["ldap_required_groups"], func(r rune) bool { return r == 10 || r == 13 || r == 59 }) {
		if g = strings.TrimSpace(g); g != "" {
			s.RequiredGroups = append(s.RequiredGroups, g)
		}
	}
	return s
}

// LDAPLogin 用 LDAP 验证用户名密码，成功返回用户属性（DN、用户名、邮箱）
func LDAPLogin(s LDAPSettings, username, password string) (dn, email string, err error) {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	var conn *goldap.Conn
	if s.TLS {
		conn, err = goldap.DialTLS("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	} else {
		conn, err = goldap.Dial("tcp", addr)
	}
	if err != nil {
		return "", "", fmt.Errorf("LDAP 连接失败: %w", err)
	}
	defer conn.Close()
	conn.SetTimeout(ldapTimeout)

	// 管理员绑定（可选；未配置则匿名搜索）
	if s.BindDN != "" {
		if err := conn.Bind(s.BindDN, s.BindPassword); err != nil {
			return "", "", fmt.Errorf("LDAP 绑定账号失败: %w", err)
		}
	}

	filter := strings.ReplaceAll(s.UserFilter, "%s", goldap.EscapeFilter(username))
	search := goldap.NewSearchRequest(
		s.BaseDN, goldap.ScopeWholeSubtree, goldap.NeverDerefAliases, 1, 0, false,
		filter, []string{s.AttrUsername, "mail", "cn"}, nil,
	)
	res, err := conn.Search(search)
	if err != nil {
		return "", "", fmt.Errorf("LDAP 搜索用户失败: %w", err)
	}
	if len(res.Entries) != 1 {
		return "", "", fmt.Errorf("LDAP 中未找到唯一用户 %s", username)
	}
	entry := res.Entries[0]
	// 用用户 DN 验证密码
	if err := conn.Bind(entry.DN, password); err != nil {
		return "", "", fmt.Errorf("LDAP 密码验证失败")
	}
	// 用户组校验：必须属于允许登录的用户组之一
	if s.GroupCheck {
		if err := checkLDAPGroup(s, conn, entry.DN); err != nil {
			return "", "", err
		}
	}
	email = entry.GetAttributeValue("mail")
	return entry.DN, email, nil
}

// ErrLDAPGroupDenied 用户不在允许登录的 LDAP 用户组中
var ErrLDAPGroupDenied = fmt.Errorf("该账号不属于允许登录的 LDAP 用户组")

// checkLDAPGroup 在组 Base DN 下用过滤器搜索用户的组，命中 RequiredGroups 之一（按 DN 或 CN 比对）才放行
// LDAPSyncEmails 管理员凭据搜索全部含 mail 属性的用户，返回 用户名 -> 邮箱
func LDAPSyncEmails(s LDAPSettings) (map[string]string, error) {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	var conn *goldap.Conn
	var err error
	if s.TLS {
		conn, err = goldap.DialTLS("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	} else {
		conn, err = goldap.Dial("tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("LDAP 连接失败: %w", err)
	}
	defer conn.Close()
	conn.SetTimeout(ldapTimeout)
	if s.BindDN != "" {
		if err := conn.Bind(s.BindDN, s.BindPassword); err != nil {
			return nil, fmt.Errorf("LDAP 绑定账号失败: %w", err)
		}
	}
	// 搜索所有含 mail 属性的条目
	search := goldap.NewSearchRequest(
		s.BaseDN, goldap.ScopeWholeSubtree, goldap.NeverDerefAliases, 0, 0, false,
		"(&(mail=*)("+s.AttrUsername+"=*))", []string{s.AttrUsername, "mail"}, nil,
	)
	res, err := conn.Search(search)
	if err != nil {
		return nil, fmt.Errorf("LDAP 搜索失败: %w", err)
	}
	out := map[string]string{}
	for _, e := range res.Entries {
		name := e.GetAttributeValue(s.AttrUsername)
		mail := e.GetAttributeValue("mail")
		if name != "" && mail != "" {
			out[name] = mail
		}
	}
	return out, nil
}

func checkLDAPGroup(s LDAPSettings, conn *goldap.Conn, userDN string) error {
	if len(s.RequiredGroups) == 0 {
		return nil // 未配置允许组则不限制
	}
	filter := strings.ReplaceAll(s.GroupFilter, "%s", goldap.EscapeFilter(userDN))
	search := goldap.NewSearchRequest(
		s.GroupBaseDN, goldap.ScopeWholeSubtree, goldap.NeverDerefAliases, 0, 0, false,
		filter, []string{"cn", "dn"}, nil,
	)
	res, err := conn.Search(search)
	if err != nil {
		return fmt.Errorf("LDAP 用户组搜索失败: %w", err)
	}
	for _, e := range res.Entries {
		dn := strings.ToLower(e.DN)
		cn := strings.ToLower(e.GetAttributeValue("cn"))
		for _, req := range s.RequiredGroups {
			r := strings.ToLower(strings.TrimSpace(req))
			if dn == r || cn == r || strings.HasSuffix(dn, ","+r) {
				return nil // 命中允许组
			}
		}
	}
	return ErrLDAPGroupDenied
}

// TestLDAP 管理员测试 LDAP 连通性
func TestLDAP(s LDAPSettings) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	var conn *goldap.Conn
	var err error
	if s.TLS {
		conn, err = goldap.DialTLS("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	} else {
		conn, err = goldap.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer conn.Close()
	if s.BindDN != "" {
		if err := conn.Bind(s.BindDN, s.BindPassword); err != nil {
			return fmt.Errorf("绑定账号失败: %w", err)
		}
	} else {
		if err := conn.UnauthenticatedBind(""); err != nil {
			return fmt.Errorf("匿名绑定失败: %w", err)
		}
	}
	return nil
}
