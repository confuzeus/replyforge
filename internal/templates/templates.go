package templates

import (
	"embed"
	"html/template"
)

//go:embed admin.html
var FS embed.FS

var AdminPage = template.Must(template.ParseFS(FS, "admin.html"))
