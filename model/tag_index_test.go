package model

import (
	"reflect"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

// Removing a covering column, reversing its priority, or introducing uniqueness
// must fail: existing sites can contain duplicate article/tag associations.
func TestTagLookupIndexes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		model   any
		columns []string
	}{
		{"tag articles", &TagData{}, []string{"tag_id", "item_id"}},
		{"article tags", &TagData{}, []string{"item_id", "tag_id"}},
		{"tag title", &Tag{}, []string{"title"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := schema.Parse(tc.model, &sync.Map{}, schema.NamingStrategy{})
			if err != nil {
				t.Fatal(err)
			}
			for _, index := range s.ParseIndexes() {
				var columns []string
				for _, field := range index.Fields {
					columns = append(columns, field.DBName)
				}
				if reflect.DeepEqual(columns, tc.columns) && index.Class == "" {
					return
				}
			}
			t.Fatalf("missing non-unique lookup index on %v", tc.columns)
		})
	}
}

func TestTagLegacyIndexesRemain(t *testing.T) {
	s, err := schema.Parse(&TagData{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range s.ParseIndexes() {
		if index.Class == "UNIQUE" {
			t.Fatalf("association indexes must preserve duplicates: %s", index.Name)
		}
	}
	for _, column := range []string{"tag_id", "item_id"} {
		found := false
		for _, index := range s.ParseIndexes() {
			if len(index.Fields) == 1 && index.Fields[0].DBName == column && index.Class == "" {
				found = true
			}
		}
		if !found {
			t.Errorf("legacy index on %s must remain", column)
		}
	}
}
