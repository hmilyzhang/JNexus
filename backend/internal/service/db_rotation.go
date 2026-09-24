// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// Database account password rotation: a source designates ONE admin account
// (is_rotator) that has the ALTER USER / ALTER LOGIN privilege; it rotates the
// passwords of the other enabled accounts of the same source. The new password
// is stored AES-encrypted so the workbench keeps working. The rotator itself
// never rotates (its stored credential would go stale).

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
)

// sqlQuoteEscape doubles single quotes inside string literals
func sqlQuoteEscape(s string) string { return strings.ReplaceAll(s, "'", "''") }

// dbAlterPasswordSQL builds the ALTER statement(s) for one dialect.
// MySQL may return several statements (one per host entry in mysql.user).
func dbAlterPasswordSQL(dbType, username, password string, extra map[string]string) ([]string, error) {
	q := sqlQuoteEscape(password)
	switch dbType {
	case "pgsql":
		u := strings.ReplaceAll(username, `"`, `""`)
		return []string{fmt.Sprintf(`ALTER ROLE "%s" WITH PASSWORD '%s'`, u, q)}, nil
	case "mssql":
		u := strings.ReplaceAll(username, "]", "]]")
		return []string{fmt.Sprintf("ALTER LOGIN [%s] WITH PASSWORD = '%s'", u, q)}, nil
	case "oracle":
		u := strings.ReplaceAll(username, `"`, `""`)
		p := strings.ReplaceAll(password, `"`, `""`)
		return []string{fmt.Sprintf(`ALTER USER "%s" IDENTIFIED BY "%s"`, u, p)}, nil
	case "mysql":
		hosts, ok := extra["hosts"]
		if !ok || hosts == "" {
			return nil, fmt.Errorf("mysql 未找到账号的 host 信息")
		}
		u := strings.ReplaceAll(username, "'", "''")
		u = strings.ReplaceAll(u, "\\", "\\\\")
		stmts := []string{}
		for _, h := range strings.Split(hosts, ",") {
			h = strings.ReplaceAll(strings.TrimSpace(h), "'", "''")
			stmts = append(stmts, fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s'", u, h, q))
		}
		return stmts, nil
	}
	return nil, fmt.Errorf("unsupported db_type %q", dbType)
}

// RotateDBAccountNow rotates one database account via the source's designated
// rotator account. Returns the new password length for logging (never the password).
func RotateDBAccountNow(accID uint, operator string) error {
	var acc model.DBAccount
	if err := model.DB.First(&acc, accID).Error; err != nil {
		return fmt.Errorf("账号不存在")
	}
	var src model.DbSource
	if err := model.DB.First(&src, acc.SourceID).Error; err != nil {
		return fmt.Errorf("数据库源不存在")
	}
	if acc.IsRotator {
		return fmt.Errorf("轮换管理账号不参与自动轮换（否则存储的凭据会失效）")
	}

	var rotator model.DBAccount
	if err := model.DB.Where("source_id = ? AND is_rotator = ?", acc.SourceID, true).
		First(&rotator).Error; err != nil {
		return fmt.Errorf("该数据库源未指定轮换管理账号（在「账号轮换」中勾选一个具备改密权限的账号）")
	}

	plainRot, err := pkg.Decrypt(rotator.Password)
	if err != nil {
		return fmt.Errorf("管理账号密码解密失败: %w", err)
	}
	newPwd, err := GenerateStrongPassword(GetRotationPolicy())
	if err != nil {
		return err
	}

	db, err := OpenDB(&src, rotator.Username, plainRot)
	if err != nil {
		return fmt.Errorf("管理账号连接失败: %w", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), DBSourceTimeout(&src))
	defer cancel()

	var stmts []string
	if src.DBType == "mysql" {
		var host sql.NullString
		if err := db.QueryRowContext(ctx,
			"SELECT COALESCE(GROUP_CONCAT(host), '') FROM mysql.user WHERE user = ?", acc.Username).
			Scan(&host); err != nil || host.String == "" {
			return fmt.Errorf("mysql.user 中未找到账号 %s 的 host 记录", acc.Username)
		}
		stmts, err = dbAlterPasswordSQL(src.DBType, acc.Username, newPwd, map[string]string{"hosts": host.String})
		if err != nil {
			return err
		}
	} else {
		stmts, err = dbAlterPasswordSQL(src.DBType, acc.Username, newPwd, nil)
		if err != nil {
			return err
		}
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("改密执行失败: %w", err)
		}
	}

	plainNew, err := pkg.Encrypt(newPwd)
	if err != nil {
		return err
	}
	now := time.Now()
	result := fmt.Sprintf("轮换成功（由管理账号 %s 执行）", rotator.Username)
	model.DB.Model(&acc).Updates(map[string]any{
		"password": plainNew, "last_rotated_at": now, "last_rotation_result": result,
	})
	model.DB.Create(&model.AuditLog{
		Action: "DB_ROTATE", Resource: fmt.Sprintf("%s@%s", acc.Username, src.Name),
		Detail: fmt.Sprintf(`{"by":%q,"operator":%q}`, rotator.Username, operator),
		Status: 200, CreatedAt: now,
	})
	return nil
}

// RunDueDBAccountRotations rotates every enabled database account whose period
// has elapsed (monitor loop). Accounts are skipped when their source has no
// designated rotator — the failure is recorded on the account itself.
func RunDueDBAccountRotations() {
	policy := GetRotationPolicy()
	var accs []model.DBAccount
	model.DB.Where("rotate_enabled = ? AND is_rotator = ?", true, false).Find(&accs)
	now := time.Now()
	for _, acc := range accs {
		days := acc.RotateDays
		if days <= 0 {
			days = policy.Days
		}
		due := acc.LastRotatedAt == nil || now.Sub(*acc.LastRotatedAt) >= time.Duration(days)*24*time.Hour
		if !due {
			continue
		}
		a := acc
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println("[db-rotation] panic:", r)
				}
			}()
			if err := RotateDBAccountNow(a.ID, "scheduler"); err != nil {
				model.DB.Model(&a).Updates(map[string]any{
					"last_rotation_result": "轮换失败: " + err.Error(),
				})
				fmt.Println("[db-rotation]", a.Username, "failed:", err.Error())
			}
		}()
	}
}
