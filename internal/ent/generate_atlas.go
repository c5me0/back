//go:build linux

package ent

//go:generate go generate generate_ddl_linux.go
//go:generate atlas migrate diff baseline --dir "file://migrate/migrations" --to "file://migrate/schema.sql" --to "file://migrate/extra.sql" --dev-url "docker://postgres/18/dev?search_path=public" --format "{{sql . \"  \"}}"
