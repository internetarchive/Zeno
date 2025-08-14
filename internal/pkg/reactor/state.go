package reactor

import "github.com/internetarchive/Zeno/pkg/models"

// GetStateTable returns a slice of all the seeds UUIDs as string in the state table.
func GetStateTable() []string {
	keys := []string{}
	globalReactor.stateTable.Range(func(key, _ any) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}

// GetStateTableItems returns a slice of all the seeds in the state table.
func GetStateTableItems() []*models.Item {
	items := []*models.Item{}
	globalReactor.stateTable.Range(func(_, value any) bool {
		items = append(items, value.(*models.Item))
		return true
	})
	return items
}
