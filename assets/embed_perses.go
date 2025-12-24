package assets

import (
	"embed"
	"io/fs"
)

// PersesDashboardsFS contains the embedded Perses dashboard YAML files.
//
//go:embed dashboards/perses/dashboards/*.yaml
var persesDashboardsFS embed.FS

// PersesDatasourcesFS contains the embedded Perses datasource YAML files.
//
//go:embed dashboards/perses/datasources/*.yaml
var persesDatasourcesFS embed.FS

// GetPersesDashboardsFS exposes the embedded FS as a read-only fs.FS.
func GetPersesDashboardsFS() fs.FS {
	return persesDashboardsFS
}

// GetPersesDatasourcesFS exposes the embedded FS as a read-only fs.FS.
func GetPersesDatasourcesFS() fs.FS {
	return persesDatasourcesFS
}
