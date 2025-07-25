package log

func hasKey(record map[string]string, key string) bool {
	_, ok := record[key]
	return ok
}
