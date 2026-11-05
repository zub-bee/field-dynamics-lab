package assets

import (
	"embed"
	"io/fs"
)

//go:embed "migrations"
var files embed.FS

var (
	MigrationFiles = sub(files, "migrations")
)

func sub(f fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
