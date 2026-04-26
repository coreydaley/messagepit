package storage

import (
	"fmt"
	"path"
	"strings"
	"testing"
)

func TestSchemaEmbedContents(t *testing.T) {
	files, err := schemaScripts.ReadDir("schemas")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	found2_0_0 := false
	for _, f := range files {
		t.Logf("File: %s (type=%s)", f.Name(), f.Type())
		if f.Name() == "2.0.0.sql" {
			found2_0_0 = true
			b, err := schemaScripts.ReadFile(path.Join("schemas", f.Name()))
			if err != nil {
				t.Fatalf("ReadFile failed: %v", err)
			}
			t.Logf("Content of 2.0.0.sql:\n%s", string(b))
		}
	}

	if !found2_0_0 {
		t.Fatal("2.0.0.sql NOT FOUND in embed FS!")
	}
}

func TestSchemaTableExists(t *testing.T) {
	setup("")
	defer Close()

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='sms_mailbox'").Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if count != 1 {
		// Let's see what tables DO exist
		rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
		if err != nil {
			t.Fatalf("query tables failed: %v", err)
		}
		defer rows.Close()
		var tables []string
		for rows.Next() {
			var name string
			rows.Scan(&name)
			tables = append(tables, name)
		}
		t.Fatalf("sms_mailbox table does NOT exist. Tables present: %s", strings.Join(tables, ", "))
	}

	// Also check which schema versions were applied
	rows, err := db.Query("SELECT Version FROM schemas ORDER BY Version")
	if err != nil {
		t.Fatalf("query schemas failed: %v", err)
	}
	defer rows.Close()
	var versions []string
	for rows.Next() {
		var v string
		rows.Scan(&v)
		versions = append(versions, v)
	}
	t.Logf("Applied schemas: %s", strings.Join(versions, ", "))
	fmt.Printf("Applied schemas: %v\n", versions)
}
