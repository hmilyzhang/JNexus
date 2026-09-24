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
	"net/url"
	"regexp"
	"strings"
	"time"

	"jnexus/internal/model"

	"github.com/go-sql-driver/mysql"
	go_ora "github.com/sijms/go-ora/v2"
)

// OpenDB opens a connection for the given source using a specific account.
func OpenDB(d *model.DbSource, username, password string) (*sql.DB, error) {
	return OpenDBFor(d, username, password, d.Database)
}

// OpenDBFor opens a connection to a specific database of the source (tree
// browsing and queries may target a database other than the registered
// default). Oracle is service-bound there the database acts as the service.
func OpenDBFor(d *model.DbSource, username, password, database string) (*sql.DB, error) {
	driver, dsn, err := buildDSN(d.DBType, d.Host, d.Port, database, username, password)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(2 * time.Minute)
	db.SetMaxOpenConns(1)
	return db, nil
}

// buildDSN constructs an escaped connection string per dialect. All URL-based
// dialects go through net/url so special characters in passwords are safe.
func buildDSN(dbType, host string, port int, database, username, password string) (string, string, error) {
	switch dbType {
	case "mysql":
		cfg := mysql.NewConfig()
		cfg.User = username
		cfg.Passwd = password
		cfg.Net = "tcp"
		cfg.Addr = fmt.Sprintf("%s:%d", host, port)
		cfg.DBName = database
		cfg.Timeout = 10 * time.Second
		return "mysql", cfg.FormatDSN(), nil
	case "mssql":
		u := url.URL{Scheme: "sqlserver", Host: fmt.Sprintf("%s:%d", host, port)}
		u.User = url.UserPassword(username, password)
		q := url.Values{}
		q.Set("database", database)
		u.RawQuery = q.Encode()
		return "sqlserver", u.String(), nil
	case "pgsql":
		u := url.URL{Scheme: "postgres", Host: fmt.Sprintf("%s:%d", host, port)}
		if database != "" {
			u.Path = "/" + database
		}
		u.User = url.UserPassword(username, password)
		u.RawQuery = "connect_timeout=10&sslmode=disable"
		return "pgx", u.String(), nil
	case "oracle":
		// pure-Go Oracle driver (no Instant Client needed); BuildUrl escapes
		// user/password/service so special characters are safe
		return "oracle", go_ora.BuildUrl(host, port, database, username, password, nil), nil
	}
	return "", "", fmt.Errorf("unsupported db_type %q", dbType)
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

var oracleWithSelectRe = regexp.MustCompile(`(?i)\bselect\b`)
var oracleWithDMLRe = regexp.MustCompile(`(?i)\b(insert|update|delete|merge)\b`)

func isReading(dbType, kind, sqlText string) bool {
	if kind == "SELECT" || kind == "SHOW" || kind == "EXPLAIN" || kind == "DESC" || kind == "DESCRIBE" {
		return true
	}
	if dbType == "oracle" && kind == "WITH" {
		upper := strings.ToUpper(sqlText)
		return oracleWithSelectRe.MatchString(upper) && !oracleWithDMLRe.MatchString(upper)
	}
	return false
}

// ValidateDBSQL applies the workbench guardrails to a statement:
// read-only sources only accept reading keywords; danger rules always apply.
func ValidateDBSQL(sqlText string, readOnly bool, dbType string) error {
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
		if isReading(dbType, kind, sqlText) {
			return nil
		}
		return fmt.Errorf("该数据库源为只读模式，仅允许查询语句")
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
	if err := ValidateDBSQL(sqlText, d.ReadOnly, d.DBType); err != nil {
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
	reading := isReading(d.DBType, kind, sqlText)
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
