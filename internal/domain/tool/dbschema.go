package tool

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ColumnSchema describes a single column in a table.
type ColumnSchema struct {
	Name     string `json:"name"`
	TypeName string `json:"typeName"`
	Size     int    `json:"size"`
	Scale    int    `json:"scale"`
	Nullable bool   `json:"nullable"`
}

// TableSchema describes a table with its columns.
type TableSchema struct {
	Name    string         `json:"name"`
	Schema  string         `json:"schema"`
	Type    string         `json:"type"`
	Columns []ColumnSchema `json:"columns"`
}

// DatabaseSchema is the full schema of a database.
type DatabaseSchema struct {
	Tables []TableSchema `json:"tables"`
}

// fetchPostgresSchema connects to the given DSN and queries information_schema for the DB schema.
func fetchPostgresSchema(ctx context.Context, dsn string) (DatabaseSchema, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return DatabaseSchema{}, fmt.Errorf("dbschema: connect: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	// Query tables and views from information_schema.
	tableRows, err := conn.Query(ctx, `
		SELECT table_schema, table_name, table_type
		FROM information_schema.tables
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name
	`)
	if err != nil {
		return DatabaseSchema{}, fmt.Errorf("dbschema: list tables: %w", err)
	}
	defer tableRows.Close()

	type tableKey struct{ schema, name string }
	var tableOrder []tableKey
	tableTypes := map[tableKey]string{}
	for tableRows.Next() {
		var schema, name, tableType string
		if err := tableRows.Scan(&schema, &name, &tableType); err != nil {
			return DatabaseSchema{}, fmt.Errorf("dbschema: scan table row: %w", err)
		}
		k := tableKey{schema, name}
		tableOrder = append(tableOrder, k)
		tableTypes[k] = tableType
	}
	if err := tableRows.Err(); err != nil {
		return DatabaseSchema{}, fmt.Errorf("dbschema: table rows error: %w", err)
	}
	tableRows.Close()

	// Query columns.
	colRows, err := conn.Query(ctx, `
		SELECT table_schema, table_name, column_name, udt_name,
		       COALESCE(character_maximum_length, numeric_precision, 0),
		       COALESCE(numeric_scale, 0),
		       is_nullable = 'YES'
		FROM information_schema.columns
		WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY table_schema, table_name, ordinal_position
	`)
	if err != nil {
		return DatabaseSchema{}, fmt.Errorf("dbschema: list columns: %w", err)
	}
	defer colRows.Close()

	colsByTable := map[tableKey][]ColumnSchema{}
	for colRows.Next() {
		var tableSchema, tableName, colName, colType string
		var size, scale int
		var nullable bool
		if err := colRows.Scan(&tableSchema, &tableName, &colName, &colType, &size, &scale, &nullable); err != nil {
			return DatabaseSchema{}, fmt.Errorf("dbschema: scan column row: %w", err)
		}
		k := tableKey{tableSchema, tableName}
		colsByTable[k] = append(colsByTable[k], ColumnSchema{
			Name:     colName,
			TypeName: colType,
			Size:     size,
			Scale:    scale,
			Nullable: nullable,
		})
	}
	if err := colRows.Err(); err != nil {
		return DatabaseSchema{}, fmt.Errorf("dbschema: column rows error: %w", err)
	}

	tables := make([]TableSchema, 0, len(tableOrder))
	for _, k := range tableOrder {
		cols := colsByTable[k]
		if cols == nil {
			cols = []ColumnSchema{}
		}
		t := "TABLE"
		if tableTypes[k] == "VIEW" {
			t = "VIEW"
		}
		tables = append(tables, TableSchema{
			Name:    k.name,
			Schema:  k.schema,
			Type:    t,
			Columns: cols,
		})
	}

	return DatabaseSchema{Tables: tables}, nil
}
