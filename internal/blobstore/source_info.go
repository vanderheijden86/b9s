package blobstore

import (
	"fmt"

	"github.com/vanderheijden86/b9s/internal/datasource"
)

// SourceInfoFromDataSource converts the data source b9s is actually reading
// into the SourceInfo Open needs to select a store. It lives in this
// package, not in internal/datasource, because pkg/ui needs it (Tasks 5 and
// 6) and cannot import cmd/b9s; blobstore already depends on pkg/config,
// which itself imports internal/datasource, so a further blobstore ->
// datasource import closes no cycle.
func SourceInfoFromDataSource(source datasource.DataSource, beadsDir, projectName string) (SourceInfo, error) {
	switch source.Type {
	case datasource.SourceTypeDolt:
		database := source.Database
		if database == "" {
			database = "beads"
		}
		return SourceInfo{
			Kind:        SourceDolt,
			Database:    database,
			BeadsDir:    beadsDir,
			ProjectName: projectName,
		}, nil
	case datasource.SourceTypeSQLite:
		return SourceInfo{Kind: SourceSQLite, BeadsDir: beadsDir, ProjectName: projectName}, nil
	case datasource.SourceTypeJSONLLocal, datasource.SourceTypeJSONLWorktree:
		return SourceInfo{Kind: SourceJSONL, BeadsDir: beadsDir, ProjectName: projectName}, nil
	default:
		return SourceInfo{}, fmt.Errorf("attachments: unsupported source type %q", source.Type)
	}
}
