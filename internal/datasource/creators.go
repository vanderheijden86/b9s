package datasource

import (
	"database/sql"

	"github.com/vanderheijden86/beadwork/pkg/debug"
	"github.com/vanderheijden86/beadwork/pkg/model"
)

// creatorQueries run in order until one succeeds. Older bd schemas lack
// owner, and the oldest lack created_by too; a missing column must never cost
// the rest of the issue data, so creators load apart from the main query.
var creatorQueries = []string{
	`SELECT id, created_by, owner FROM issues`,
	`SELECT id, created_by, NULL FROM issues`,
}

// applyCreators fills CreatedBy and Owner on issues from the issues table.
// A schema without either column leaves the issues unchanged.
func applyCreators(db *sql.DB, issues []model.Issue) {
	if len(issues) == 0 {
		return
	}
	index := make(map[string]int, len(issues))
	for i := range issues {
		index[issues[i].ID] = i
	}
	for _, query := range creatorQueries {
		rows, err := db.Query(query)
		if err != nil {
			debug.Log("creators: %q failed: %v", query, err)
			continue
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var createdBy, owner sql.NullString
			if err := rows.Scan(&id, &createdBy, &owner); err != nil {
				continue
			}
			i, ok := index[id]
			if !ok {
				continue
			}
			issues[i].CreatedBy = createdBy.String
			issues[i].Owner = owner.String
		}
		return
	}
}
