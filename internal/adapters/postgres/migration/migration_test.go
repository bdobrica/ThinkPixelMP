package migration

import (
	"testing"
	"testing/fstest"
)

func TestLoadRejectsInvalidMigrationSets(t *testing.T) {
	for _, files := range []fstest.MapFS{
		{}, {"001_bad.sql": {Data: []byte("SELECT 1")}},
		{"000002_gap.sql": {Data: []byte("SELECT 1")}},
		{"000001_empty.sql": {}},
		{"000001_a.sql": {Data: []byte("SELECT 1")}, "000001_b.sql": {Data: []byte("SELECT 2")}},
	} {
		if _, err := load(files); err == nil {
			t.Fatal("accepted invalid migration set")
		}
	}
}
