// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// Database workbench (PAM phase B1): run day-to-day SQL against registered
// database sources, as one of their accounts. Guardrails: per-source read-only
// mode, dangerous-SQL interception, statement timeout and a row cap; every
// execution is audited by the handler.

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"jnexus/internal/model"
)

// OpenDB opens a connection for the given source using a specific account.
func OpenDB(d *model.DbSource, username, password string) (*sql.DB, error) {
	var driver, dsn string
	switch d.DBType {
	case "mysql":
		driver = "mysql"
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=10s", username, password, d.Host, d.Port, d.Database)
	case "mssql":
		driver = "sqlserver"
		dsn = fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s", username, password, d.Host, d.Port, d.Database)
	case "pgsql":
		driver = "pgx"
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?connect_timeout=10&sslmode=disable", username, password, d.Host, d.Port, d.Database)
	default:
		return nil, fmt.Errorf("unsupported db_type %q", d.DBType)
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(2 * time.Minute)
	db.SetMaxOpenConns(1)
	return db, nil
}

func DBSourceTimeout(d *model.DbSource) time.Duration {
	if d.TimeoutSec > 0 {
		return time.Duration(d.TimeoutSec) * time.Second
	}
	return 30 * time.Second
}

func DBSourceRowLimit(d *model.DbSource) int {
	if d.MaxRows > 0 {
		return d.MaxRows
	}
	return 1000
}

var leadingCommentRe = regexp.MustCompile(`(?s)^\s*(/\*.*?\*/|--[^\n]*\n)*\s*`)

// Built-in DB-specific danger patterns (on top of the shared danger-rule
// library): a workbench statement must never destroy a whole database/schema
// or wipe a table in one shot. DROP TABLE/INDEX stay allowed (legitimate DBA work).
var dbDangerPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bdrop\s+(database|schema)\b`),
	regexp.MustCompile(`(?i)\btruncate\s+(table\s+)?`),
	regexp.MustCompile(`(?is)\bdelete\s+from\s+[^\s]+(\s+as\s+\w+)?\s*(;|$)`), // DELETE without WHERE
}

// statementKind returns the first SQL keyword of the statement (comments stripped)
func statementKind(sqlText string) string {
	trimmed := leadingCommentRe.ReplaceAllString(sqlText, "")
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToUpper(fields[0])
}

// ValidateDBSQL applies the workbench guardrails to a statement:
// read-only sources only accept reading keywords; danger rules always apply.
func ValidateDBSQL(sqlText string, readOnly bool) error {
	kind := statementKind(sqlText)
	if kind == "" {
		return fmt.Errorf("SQL 语句为空")
	}
	if hits := CheckDanger(sqlText); len(hits) > 0 {
		return fmt.Errorf("危险语句已被拦截: %s", strings.Join(hits, "、"))
	}
	for _, re := range dbDangerPatterns {
		if re.MatchString(sqlText) {
			return fmt.Errorf("危险语句已被拦截（数据库保护规则）")
		}
	}
	if readOnly {
		switch kind {
		case "SELECT", "SHOW", "EXPLAIN", "DESC", "DESCRIBE":
			return nil
		default:
			return fmt.Errorf("该数据库源为只读模式，仅允许 SELECT / SHOW / EXPLAIN / DESC")
		}
	}
	return nil
}

// DBColumn describes one result column (name + database type name)
type DBColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// DBQueryResult is the workbench result payload (rows are stringified for JSON)
type DBQueryResult struct {
	Columns   []DBColumn      `json:"columns"`
	Rows      [][]interface{} `json:"rows"`
	Affected  int64           `json:"affected"`
	Elapsed   int64           `json:"elapsed_ms"`
	Truncated bool            `json:"truncated"`
}

// RunDBQuery executes one statement as the given account with the source's
// guardrails and returns the result grid. Uses Query for reading keywords and
// Exec for everything else (drivers reject Exec on result-returning statements).
func RunDBQuery(d *model.DbSource, username, password, sqlText string) (*DBQueryResult, error) {
	if err := ValidateDBSQL(sqlText, d.ReadOnly); err != nil {
		return nil, err
	}
	db, err := OpenDB(d, username, password)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), DBSourceTimeout(d))
	defer cancel()
	started := time.Now()
	res := &DBQueryResult{}

	kind := statementKind(sqlText)
	reading := kind == "SELECT" || kind == "SHOW" || kind == "EXPLAIN" || kind == "DESC" || kind == "DESCRIBE"
	if reading {
		rows, qerr := db.QueryContext(ctx, sqlText)
		if qerr != nil {
			return nil, qerr
		}
		defer rows.Close()
		colTypes, cerr := rows.ColumnTypes()
		if cerr != nil {
			return nil, cerr
		}
		for _, ct := range colTypes {
			res.Columns = append(res.Columns, DBColumn{Name: ct.Name(), Type: ct.DatabaseTypeName()})
		}
		limit := DBSourceRowLimit(d)
		for rows.Next() {
			if len(res.Rows) >= limit {
				res.Truncated = true
				break
			}
			raw := make([]any, len(res.Columns))
			ptrs := make([]any, len(res.Columns))
			for i := range raw {
				ptrs[i] = &raw[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return nil, err
			}
			row := make([]interface{}, len(res.Columns))
			for i, v := range raw {
				row[i] = stringifyDBValue(v)
			}
			res.Rows = append(res.Rows, row)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	} else {
		execRes, xerr := db.ExecContext(ctx, sqlText)
		if xerr != nil {
			return nil, xerr
		}
		if n, err := execRes.RowsAffected(); err == nil {
			res.Affected = n
		}
	}
	res.Elapsed = time.Since(started).Milliseconds()
	return res, nil
}

func stringifyDBValue(v any) interface{} {
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		return string(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}
