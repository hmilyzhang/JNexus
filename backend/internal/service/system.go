// JNexus Ops Platform — By JJ Zhang, Version 1.0

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

// SystemConfigMap reads all system config into a map
func SystemConfigMap() map[string]string {
	out := map[string]string{}
	var rows []model.SystemConfig
	model.DB.Find(&rows)
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out
}

// SetSystemConfigs writes configs in batch (creates the key if absent)
func SetSystemConfigs(m map[string]string) error {
	for k, v := range m {
		if err := model.DB.Save(&model.SystemConfig{Key: k, Value: v}).Error; err != nil {
			return err
		}
	}
	return nil
}

// LDAPSettings extracts LDAP settings from system config
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
	GroupCheck     bool     // enable group check
	GroupBaseDN    string   // Base DN for group search
	GroupFilter    string   // group filter; %s is replaced with the user DN
	RequiredGroups []string // list of group DN/CN allowed to log in
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

// LDAPLogin verifies the username/password against LDAP; on success returns user attributes (DN, username, email)
func LDAPLogin(s LDAPSettings, username, password string) (dn, email, displayName string, err error) {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	var conn *goldap.Conn
	if s.TLS {
		conn, err = goldap.DialTLS("tcp", addr, &tls.Config{InsecureSkipVerify: true})
	} else {
		conn, err = goldap.Dial("tcp", addr)
	}
	if err != nil {
		return "", "", "", fmt.Errorf("LDAP 连接失败: %w", err)
	}
	defer conn.Close()
	conn.SetTimeout(ldapTimeout)

	// admin bind (optional; falls back to anonymous search when not configured)
	if s.BindDN != "" {
		if err := conn.Bind(s.BindDN, s.BindPassword); err != nil {
			return "", "", "", fmt.Errorf("LDAP 绑定账号失败: %w", err)
		}
	}

	filter := strings.ReplaceAll(s.UserFilter, "%s", goldap.EscapeFilter(username))
	search := goldap.NewSearchRequest(
		s.BaseDN, goldap.ScopeWholeSubtree, goldap.NeverDerefAliases, 1, 0, false,
		filter, []string{s.AttrUsername, "mail", "cn", "displayName"}, nil,
	)
	res, err := conn.Search(search)
	if err != nil {
		return "", "", "", fmt.Errorf("LDAP 搜索用户失败: %w", err)
	}
	if len(res.Entries) != 1 {
		return "", "", "", fmt.Errorf("LDAP 中未找到唯一用户 %s", username)
	}
	entry := res.Entries[0]
	// verify the password with the user DN
	if err := conn.Bind(entry.DN, password); err != nil {
		return "", "", "", fmt.Errorf("LDAP 密码验证失败")
	}
	// group check: the user must belong to one of the groups allowed to log in
	if s.GroupCheck {
		if err := checkLDAPGroup(s, conn, entry.DN); err != nil {
			return "", "", "", err
		}
	}
	email = entry.GetAttributeValue("mail")
	displayName = entry.GetAttributeValue("displayName")
	if displayName == "" {
		displayName = entry.GetAttributeValue("cn")
	}
	return entry.DN, email, displayName, nil
}

// ErrLDAPGroupDenied indicates the user is not in any LDAP group allowed to log in
var ErrLDAPGroupDenied = fmt.Errorf("该账号不属于允许登录的 LDAP 用户组")

// checkLDAPGroup searches the user's groups under the group Base DN with the filter; access is granted only when one of RequiredGroups matches (compared by DN or CN)
// LDAPSyncEmails searches all users with a mail attribute using admin credentials, returning username -> email
func LDAPSyncEmails(s LDAPSettings) (map[string]LDAPOpts, error) {
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
	// search all entries that have a mail attribute
	search := goldap.NewSearchRequest(
		s.BaseDN, goldap.ScopeWholeSubtree, goldap.NeverDerefAliases, 0, 0, false,
		"(&(mail=*)("+s.AttrUsername+"=*))", []string{s.AttrUsername, "mail", "displayName", "cn"}, nil,
	)
	res, err := conn.Search(search)
	if err != nil {
		return nil, fmt.Errorf("LDAP 搜索失败: %w", err)
	}
	out := map[string]LDAPOpts{}
	for _, e := range res.Entries {
		name := e.GetAttributeValue(s.AttrUsername)
		mail := e.GetAttributeValue("mail")
		dp := e.GetAttributeValue("displayName")
		if dp == "" {
			dp = e.GetAttributeValue("cn")
		}
		if name != "" && mail != "" {
			out[name] = LDAPOpts{Email: mail, DisplayName: dp}
		}
	}
	return out, nil
}

// LDAPOpts holds user attributes obtained from LDAP sync
type LDAPOpts struct {
	Email       string
	DisplayName string
}

func checkLDAPGroup(s LDAPSettings, conn *goldap.Conn, userDN string) error {
	if len(s.RequiredGroups) == 0 {
		return nil // no allowed groups configured means no restriction
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
				return nil // matched an allowed group
			}
		}
	}
	return ErrLDAPGroupDenied
}

// TestLDAP lets an admin test LDAP connectivity
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
