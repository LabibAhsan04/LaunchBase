package migrations

import _ "embed"

//go:embed 0001_initial.sql
var Initial string

const Version = "0001_initial"
