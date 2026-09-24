// JNexus Ops Platform — By JJ Zhang, Version 1.0
package handler

// Database tree browsing: databases on a source instance → schemas → tables
// (with approximate row counts) → columns. Lazy endpoints backing the
// DBeaver-style navigator; every level needs an account the user may use.

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"jnexus/internal/model"
	"jnexus/internal/pkg"
	"jnexus/internal/service"
)

// loadBrowseContext validates source+account and decrypts the password
func loadBrowseContext(c *gin.Context) (*model.DbSource, *model.DBAccount, string, bool) {
	u := currentUser(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var src model.DbSource
	if err := model.DB.First(&src, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "数据库源不存在"})
		return nil, nil, "", false
	}
	accountID, _ := strconv.Atoi(c.Query("account_id"))
	var account model.DBAccount
	if err := model.DB.First(&account, accountID).Error; err != nil || account.SourceID != src.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "账号不存在或不属于该数据库源"})
		return nil, nil, "", false
	}
	if !accountAllowedFor(u, account) {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权使用该数据库账号"})
		return nil, nil, "", false
	}
	password, err := pkg.Decrypt(account.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解密失败: " + err.Error()})
		return nil, nil, "", false
	}
	return &src, &account, password, true
}

func scanStrings(c *gin.Context, rows *sql.Rows) {
	if rows != nil {
		defer rows.Close()
	}
	out := []string{}
	if rows != nil {
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err == nil {
				out = append(out, v)
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// ListSourceDatabases GET /databases/:id/databases?account_id=N — databases on
// the instance (system databases filtered out). Oracle binds to one service:
// returns the service itself (per-user schemas are browsed instead).
func ListSourceDatabases(c *gin.Context) {
	src, account, password, ok := loadBrowseContext(c)
	if !ok {
		return
	}
	db, err := service.OpenDBFor(src, account.Username, password, src.Database)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "连接失败: " + err.Error()})
		return
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	switch src.DBType {
	case "pgsql":
		rows, err := db.QueryContext(ctx, `SELECT datname FROM pg_database WHERE datistemplate = false AND datallowconn = true ORDER BY datname`)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		scanStrings(c, rows)
	case "mysql":
		rows, err := db.QueryContext(ctx, `SELECT schema_name FROM information_schema.schemata WHERE schema_name NOT IN ('information_schema','mysql','performance_schema','sys') ORDER BY schema_name`)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		scanStrings(c, rows)
	case "mssql":
		rows, err := db.QueryContext(ctx, `SELECT name FROM sys.databases WHERE database_id > 4 ORDER BY name`)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		scanStrings(c, rows)
	case "oracle":
		c.JSON(http.StatusOK, gin.H{"items": []string{src.Database}})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的数据库类型"})
	}
}

// browseDB opens a connection to the requested database of the source
func browseDB(c *gin.Context, src *model.DbSource, account *model.DBAccount, password, database string) (*sql.DB, context.Context, context.CancelFunc, bool) {
	db, err := service.OpenDBFor(src, account.Username, password, database)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "连接失败: " + err.Error()})
		return nil, nil, nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	return db, ctx, cancel, true
}

// ListDatabaseSchemas GET /databases/:id/schemas?account_id&database
func ListDatabaseSchemas(c *gin.Context) {
	src, account, password, ok := loadBrowseContext(c)
	if !ok {
		return
	}
	database := c.Query("database")
	db, ctx, cancel, ok := browseDB(c, src, account, password, database)
	if !ok {
		return
	}
	defer db.Close()
	defer cancel()
	switch src.DBType {
	case "pgsql":
		rows, err := db.QueryContext(ctx, `SELECT nspname FROM pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname <> 'information_schema' ORDER BY nspname`)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		scanStrings(c, rows)
	case "mysql":
		// MySQL has no schema level distinct from the database
		c.JSON(http.StatusOK, gin.H{"items": []string{database}})
	case "mssql":
		rows, err := db.QueryContext(ctx, `SELECT name FROM sys.schemas WHERE name NOT IN ('db_accessadmin','db_backupoperator','db_datareader','db_datawriter','db_ddladmin','db_denydatareader','db_denydatawriter','db_owner','db_securityadmin','guest','INFORMATION_SCHEMA','sys') ORDER BY name`)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		scanStrings(c, rows)
	case "oracle":
		rows, err := db.QueryContext(ctx, `SELECT username FROM all_users ORDER BY username`)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		scanStrings(c, rows)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的数据库类型"})
	}
}

// ListDatabaseTables GET /databases/:id/tables?account_id&database&schema —
// base tables with approximate row counts
func ListDatabaseTables(c *gin.Context) {
	src, account, password, ok := loadBrowseContext(c)
	if !ok {
		return
	}
	database := c.Query("database")
	schema := c.Query("schema")
	db, ctx, cancel, ok := browseDB(c, src, account, password, database)
	if !ok {
		return
	}
	defer db.Close()
	defer cancel()
	type trow struct {
		Name string
		Rows int64
	}
	out := []gin.H{}
	switch src.DBType {
	case "pgsql":
		rows, err := db.QueryContext(ctx, `SELECT c.relname, GREATEST(c.reltuples, 0)::bigint FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND c.relkind = 'r' ORDER BY c.relname`, schema)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var t trow
			if rows.Scan(&t.Name, &t.Rows) == nil {
				out = append(out, gin.H{"name": t.Name, "rows": t.Rows})
			}
		}
	case "mysql":
		rows, err := db.QueryContext(ctx, `SELECT table_name, IFNULL(table_rows, 0) FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE' ORDER BY table_name`, database)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var t trow
			if rows.Scan(&t.Name, &t.Rows) == nil {
				out = append(out, gin.H{"name": t.Name, "rows": t.Rows})
			}
		}
	case "mssql":
		rows, err := db.QueryContext(ctx, `SELECT t.name, COALESCE(SUM(p.rows), 0) FROM sys.tables t LEFT JOIN sys.partitions p ON p.object_id = t.object_id AND p.index_id IN (0, 1) WHERE SCHEMA_NAME(t.schema_id) = @p1 GROUP BY t.name ORDER BY t.name`, sql.Named("p1", schema))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var t trow
			if rows.Scan(&t.Name, &t.Rows) == nil {
				out = append(out, gin.H{"name": t.Name, "rows": t.Rows})
			}
		}
	case "oracle":
		rows, err := db.QueryContext(ctx, `SELECT table_name, NVL(num_rows, 0) FROM all_tables WHERE owner = :1 ORDER BY table_name`, schema)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var t trow
			if rows.Scan(&t.Name, &t.Rows) == nil {
				out = append(out, gin.H{"name": t.Name, "rows": t.Rows})
			}
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的数据库类型"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// ListTableColumns GET /databases/:id/columns?account_id&database&schema&table
func ListTableColumns(c *gin.Context) {
	src, account, password, ok := loadBrowseContext(c)
	if !ok {
		return
	}
	database := c.Query("database")
	schema := c.Query("schema")
	table := c.Query("table")
	db, ctx, cancel, ok := browseDB(c, src, account, password, database)
	if !ok {
		return
	}
	defer db.Close()
	defer cancel()
	type crow struct {
		Name string
		Type string
	}
	out := []gin.H{}
	var rows *sql.Rows
	var err error
	if src.DBType == "oracle" {
		rows, err = db.QueryContext(ctx, `SELECT column_name, data_type FROM all_tab_columns WHERE owner = :1 AND table_name = :2 ORDER BY column_id`, schema, table)
	} else if src.DBType == "pgsql" {
		rows, err = db.QueryContext(ctx, `SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`, schema, table)
	} else {
		rows, err = db.QueryContext(ctx, `SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position`, schema, table)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	for rows.Next() {
		var cr crow
		if rows.Scan(&cr.Name, &cr.Type) == nil {
			out = append(out, gin.H{"name": cr.Name, "type": cr.Type})
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}
