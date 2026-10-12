package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSupportFiles(t *testing.T) {
	db := mustOpen(t, t.TempDir(), nil, "v0.1.0")
	defer db.Close()
	for i := 0; i < 250; i++ {
		mustExec(t, db, `INSERT INTO order_journal(at,tag,kind,source) VALUES(?,?,?,?)`, i, "t", "intent", "manual")
	}
	mustExec(t, db, `INSERT INTO settings(key,value,updated_at) VALUES('kite.api_secret','"s3cretV4lueXYZ"',1)`)
	files, err := db.SupportFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string][]byte{}
	for _, f := range files {
		byName[f.Name] = f.Data
	}
	for _, f := range files {
		if strings.Contains(string(f.Data), "s3cretV4lueXYZ") || strings.Contains(f.Name, "secret") {
			t.Fatalf("%s leaks a secret", f.Name)
		}
	}
	var journal []journalRow
	if err := json.Unmarshal(byName["order_journal.json"], &journal); err != nil {
		t.Fatal(err)
	}
	if len(journal) != JournalTailRows || journal[0].At != 249 {
		t.Fatalf("journal tail: %d rows, first at=%d", len(journal), journal[0].At)
	}
	var v map[string]any
	json.Unmarshal(byName["versions.json"], &v)
	if v["schema"].(float64) != 1 || v["app_version_last"] != "v0.1.0" {
		t.Fatalf("versions %v", v)
	}
	if string(byName["deployment_logs.json"]) != "[]" || byName["settings.json"] == nil {
		t.Fatalf("files %v", byName)
	}
}
