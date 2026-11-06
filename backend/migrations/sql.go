package migrations

import _ "embed"

//go:embed 0001_initial.sql
var initial string

//go:embed 0002_outbox.sql
var outbox string

// File is one migration. Version is the key stored in schema_migrations.
type File struct {
	Version string
	SQL     string
}

// All returns migrations in the order they must be applied.
func All() []File {
	return []File{
		{Version: "0001_initial", SQL: initial},
		{Version: "0002_outbox", SQL: outbox},
	}
}
