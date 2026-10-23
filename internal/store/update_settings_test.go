package store

import (
	"encoding/json"
	"testing"
)

func TestLegacyUpdateIntervalNormalizesOnReadAndSave(t *testing.T) {
	s := openTest(t)
	if _, err := s.db.Exec(`INSERT INTO settings(id,json) VALUES(1,?)`, `{"autoCheckUpdate":false,"updateCheckInterval":1440,"future":true}`); err != nil {
		t.Fatal(err)
	}
	v, err := s.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if v.AutoCheckUpdate || v.UpdateCheckInterval != 60 {
		t.Fatalf("legacy settings = %+v", v)
	}
	v, err = s.MergeSettings(map[string]json.RawMessage{"updateCheckInterval": json.RawMessage(`1`)})
	if err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := s.db.QueryRow("SELECT json FROM settings WHERE id=1").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stored), &raw); err != nil {
		t.Fatal(err)
	}
	if v.UpdateCheckInterval != 60 || string(raw["updateCheckInterval"]) != "60" || string(raw["future"]) != "true" || v.AutoCheckUpdate {
		t.Fatalf("normalized save lost settings: %s", stored)
	}
}
