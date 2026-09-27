package main

import (
	"fmt"
	"net"

	"github.com/vanderheijden86/beadwork/internal/blobstore"
	"github.com/vanderheijden86/beadwork/internal/datasource"
)

// attachmentSourceInfo converts the data source b9s is actually reading into
// the blobstore.SourceInfo Open needs to select a store. It lives here, not
// in internal/datasource, because datasource sits below pkg/config (see
// pkg/config/recent.go) and blobstore sits above it (for
// config.AttachmentsConfig); a datasource->blobstore import would close that
// cycle back through config.
func attachmentSourceInfo(source datasource.DataSource, beadsDir, projectName string) (blobstore.SourceInfo, error) {
	switch source.Type {
	case datasource.SourceTypeDolt:
		host, _, err := net.SplitHostPort(source.Path)
		if err != nil {
			return blobstore.SourceInfo{}, fmt.Errorf("attachments: parse dolt server address %q: %w", source.Path, err)
		}
		database := source.Database
		if database == "" {
			database = "beads"
		}
		return blobstore.SourceInfo{
			Kind:        blobstore.SourceDolt,
			DoltHost:    host,
			Database:    database,
			BeadsDir:    beadsDir,
			ProjectName: projectName,
		}, nil
	case datasource.SourceTypeSQLite:
		return blobstore.SourceInfo{Kind: blobstore.SourceSQLite, BeadsDir: beadsDir, ProjectName: projectName}, nil
	case datasource.SourceTypeJSONLLocal, datasource.SourceTypeJSONLWorktree:
		return blobstore.SourceInfo{Kind: blobstore.SourceJSONL, BeadsDir: beadsDir, ProjectName: projectName}, nil
	default:
		return blobstore.SourceInfo{}, fmt.Errorf("attachments: unsupported source type %q", source.Type)
	}
}
