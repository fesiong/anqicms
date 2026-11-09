package model

import (
	"fmt"
	"sync"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Restoring varchar(255) on either service code must exceed the legacy InnoDB
// index limit. Removing uniqueness must also fail this contract.
func TestOrderServiceCodeLegacyIndexCompatibility(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(localhost:3306)/gorm",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name  string
		model any
	}{
		{"service definition", &OrderService{}},
		{"order service addon", &OrderServiceAddon{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := schema.Parse(tc.model, &sync.Map{}, schema.NamingStrategy{})
			if err != nil {
				t.Fatal(err)
			}
			field := s.LookUpField("service_code")
			if field == nil {
				t.Fatal("missing service_code column")
			}
			if !field.Unique || !field.NotNull {
				t.Fatal("service_code must retain its NOT NULL unique constraint")
			}
			columnType := db.Migrator().FullDataTypeOf(field).SQL
			var length int
			if _, err := fmt.Sscanf(columnType, "varchar(%d)", &length); err != nil || length <= 0 {
				t.Fatalf("expected a bounded varchar, got %q", columnType)
			}
			if bytes := length * 4; bytes > 767 {
				t.Fatalf("service_code %s needs up to %d utf8mb4 index bytes, exceeding 767", columnType, bytes)
			}
		})
	}
}
