package dao

import (
	"testing"
)

func TestAutoMigrateDB(t *testing.T) {
	AutoMigrateDB(DB)
}
