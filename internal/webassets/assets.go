package webassets

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

func FS() fs.FS   { sub, _ := fs.Sub(embedded, "dist"); return sub }
func Ready() bool { _, err := fs.Stat(FS(), "index.html"); return err == nil }
