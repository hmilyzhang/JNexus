// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

// Database ingestion sources (OpenObserve builtin): admins register MySQL / MSSQL /
// PostgreSQL connections with a custom query; a scheduler runs them on an interval and
// pushes the result rows into the configured OpenObserve stream.

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"jnexus/internal/model"
	"jnexus/internal/pkg"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/microsoft/go-mssqldb"
)

// ListDbSources returns all sources with passwords masked
func ListDbSources() []model.DbSource {
	var out []model.DbSource
	model.DB.Order("id").Find(&out)
	for i := range out {
		if out[i].Password != "" {
			out[i].Password = "******"
		}
		if out[i].IntervalSec <= 0 {
			out[i].IntervalSec = 300
		}
	}
	return out
}

// SaveDbSource creates or updates a source; masked/empty password keeps the stored one
func SaveDbSource(src *model.DbSource) error {
	src.Name = strings.TrimSpace(src.Name)
	if src.Name == "" {
		return fmt.Errorf("name is required")
	}
	switch strings.ToLower(src.DBType) {
	case "mysql":
		if src.Port == 0 {
			src.Port = 3306
		}
	case "mssql":
		if src.Port == 0 {
			src.Port = 1433
		}
	case "pgsql":
		if src.Port == 0 {
			src.Port = 5432
		}
	default:
		return fmt.Errorf("db_type must be mysql / mssql / pgsql")
	}
	src.DBType = strings.ToLower(src.DBType)
	if src.Host == "" || src.Database == "" {
		return fmt.Errorf("host and database are required")
	}
	workbenchOnly := strings.TrimSpace(src.Query) == ""
	if src.Enabled && workbenchOnly {
		return fmt.Errorf("query is required for enabled sources")
	}
	if src.IntervalSec <= 0 {
		src.IntervalSec = 300
	}
	src.Stream = strings.TrimSpace(src.Stream)
	if !workbenchOnly {
		if src.Stream == "" {
			src.Stream = "db_" + strings.NewReplacer(" ", "_", "-", "_").Replace(strings.ToLower(src.Name))
		}
		if !OOStreamNameValid(src.Stream) {
			return fmt.Errorf("invalid stream name")
		}
	}

	if src.ID > 0 {
		var stored model.DbSource
		if err := model.DB.First(&stored, src.ID).Error; err != nil {
			return fmt.Errorf("source not found")
		}
		if src.Password == "" || src.Password == "******" {
			src.Password = stored.Password // keep the stored one
		}
	}
	if src.Password != "" && src.Password != "******" {
		enc, err := pkg.Encrypt(src.Password)
		if err != nil {
			return err
		}
		src.Password = enc
	} else {
		src.Password = ""
	}
	return model.DB.Save(src).Error
}

// DeleteDbSource removes a source by id
func DeleteDbSource(id uint) error {
	return model.DB.Delete(&model.DbSource{}, id).Error
}

// SetDbSourceEnabled flips the enabled flag without touching other fields
func SetDbSourceEnabled(id uint, enabled bool) error {
	return model.DB.Model(&model.DbSource{}).Where("id = ?", id).Update("enabled", enabled).Error
}

// GetDbSourcePlain loads a source and decrypts its password for the connection
func GetDbSourcePlain(id uint) (*model.DbSource, error) {
	var d model.DbSource
	if err := model.DB.First(&d, id).Error; err != nil {
		return nil, err
	}
	plain, err := pkg.Decrypt(d.Password)
	if err != nil {
		return nil, err
	}
	d.Password = plain
	return &d, nil
}

func dbSourceOpen(d *model.DbSource) (*sql.DB, error) {
	var driver, dsn string
	switch d.DBType {
	case "mysql":
		driver = "mysql"
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=10s", d.Username, d.Password, d.Host, d.Port, d.Database)
	case "mssql":
		driver = "sqlserver"
		dsn = fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s", d.Username, d.Password, d.Host, d.Port, d.Database)
	case "pgsql":
		driver = "pgx"
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?connect_timeout=10&sslmode=disable", d.Username, d.Password, d.Host, d.Port, d.Database)
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



// CreateWorkbenchSource registers a database source for the workbench
// (enabled=false, no ingestion query - the guardrail fields apply to the workbench).
func CreateWorkbenchSource(name, dbType, host string, port int, database string, readOnly bool, timeoutSec, maxRows int) (uint, error) {
	src := &model.DbSource{
		Name: name, DBType: dbType, Host: host, Port: port, Database: database,
		ReadOnly: readOnly, TimeoutSec: timeoutSec, MaxRows: maxRows, Enabled: false,
	}
	if err := SaveDbSource(src); err != nil {
		return 0, err
	}
	return src.ID, nil
}

// RunDbSource executes the source query once and pushes the rows into the stream.
// Returns the number of rows pushed.
func RunDbSource(id uint) (int, error) {
	d, err := GetDbSourcePlain(id)
	if err != nil {
		return 0, fmt.Errorf("load source: %w", err)
	}
	if d.Query == "" {
		return 0, fmt.Errorf("query is empty")
	}
	db, err := dbSourceOpen(d)
	if err != nil {
		markDbSourceRun(d.ID, 0, err)
		return 0, err
	}
	defer db.Close()

	rows, err := db.Query(d.Query)
	if err != nil {
		markDbSourceRun(d.ID, 0, err)
		return 0, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		markDbSourceRun(d.ID, 0, err)
		return 0, err
	}
	records := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			markDbSourceRun(d.ID, 0, err)
			return 0, err
		}
		rec := map[string]any{}
		for i, c := range cols {
			v := vals[i]
			switch v := v.(type) { // normalize driver types to JSON-safe values
			case nil:
				rec[c] = nil
			case []byte:
				rec[c] = string(v)
			case time.Time:
				rec[c] = v.UTC().Format(time.RFC3339)
			default:
				rec[c] = v
			}
		}
		records = append(records, rec)
		if len(records) >= 1000 {
			break // safety cap per run
		}
	}
	if len(records) == 0 {
		markDbSourceRun(d.ID, 0, nil)
		return 0, nil
	}
	if err := OOIngestJSON(d.Stream, records); err != nil {
		markDbSourceRun(d.ID, 0, err)
		return 0, err
	}
	markDbSourceRun(d.ID, len(records), nil)
	return len(records), nil
}

var dbSourceMu sync.Mutex

func markDbSourceRun(id uint, rows int, err error) {
	dbSourceMu.Lock()
	defer dbSourceMu.Unlock()
	now := time.Now()
	upd := map[string]any{"last_run_at": now}
	if err != nil {
		msg := err.Error()
		if len(msg) > 500 {
			msg = msg[:500]
		}
		upd["last_error"] = msg
	} else {
		upd["last_error"] = ""
		upd["rows_pushed"] = rows
	}
	model.DB.Model(&model.DbSource{}).Where("id = ?", id).Updates(upd)
}

// RunDueDbSources runs every enabled source whose interval has elapsed (monitor loop)
func RunDueDbSources() {
	var sources []model.DbSource
	model.DB.Where("enabled = ?", true).Find(&sources)
	now := time.Now()
	for _, s := range sources {
		if s.LastRunAt != nil && now.Sub(*s.LastRunAt) < time.Duration(s.IntervalSec)*time.Second {
			continue
		}
		s := s
		go func() {
			defer func() { recover() }()
			if _, err := RunDbSource(s.ID); err != nil {
				fmt.Println("[db-ingest]", s.Name, "failed:", err.Error())
			}
		}()
	}
}
