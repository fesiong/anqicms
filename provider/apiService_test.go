package provider

import "testing"

func TestParseOrderBy(t *testing.T) {
	t.Log(ParseOrderBy("archives.`created_time` desc", "archives"))
	t.Log(ParseOrderBy("Rand()", "archives"))
	t.Log(ParseOrderBy("Max(id)", "archives"))
	t.Log(ParseOrderBy("Max(`id`)", "archives"))
}
