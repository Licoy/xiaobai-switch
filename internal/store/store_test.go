package store

import (
	"database/sql"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := s.Close(); e != nil {
			t.Error(e)
		}
	})
	return s
}
func createTestSite(t *testing.T, s *Store) domain.SiteDTO {
	t.Helper()
	v, e := s.CreateSite(domain.CreateSiteInput{Name: "Relay", BaseURL: "https://example.test", APIKey: "sk-one-secret"})
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func ptr[T any](v T) *T { return &v }
func TestSiteKeysTransactionsAndIsolation(t *testing.T) {
	s := openTest(t)
	v := createTestSite(t, s)
	first := *v.ActiveAPIKeyID
	if _, e := s.AddSiteAPIKey(v.ID, domain.AddSiteApiKeyInput{Label: ptr("k 1"), APIKey: "sk-two"}); e == nil {
		t.Fatal("accepted duplicate label")
	}
	if _, e := s.AddSiteAPIKey(v.ID, domain.AddSiteApiKeyInput{APIKey: "sk-one-secret"}); e == nil {
		t.Fatal("accepted duplicate secret")
	}
	v, e := s.AddSiteAPIKey(v.ID, domain.AddSiteApiKeyInput{APIKey: "sk-two-secret"})
	if e != nil {
		t.Fatal(e)
	}
	second := v.APIKeys[1].ID
	if e = s.ReplaceModels(v.ID, first, []domain.SiteModelDTO{{ModelID: "first"}}); e != nil {
		t.Fatal(e)
	}
	if e = s.ReplaceModels(v.ID, second, []domain.SiteModelDTO{{ModelID: "second"}}); e != nil {
		t.Fatal(e)
	}
	v, e = s.ActivateSiteAPIKey(v.ID, second)
	if e != nil {
		t.Fatal(e)
	}
	active := 0
	for _, k := range v.APIKeys {
		if k.IsActive {
			active++
		}
	}
	if active != 1 || *v.ActiveAPIKeyID != second {
		t.Fatalf("active invariant %+v", v.APIKeys)
	}
	models, e := s.ListModels(v.ID)
	if e != nil || len(models) != 1 || models[0].ModelID != "second" {
		t.Fatalf("models crossed keys %+v %v", models, e)
	}
	if _, e = s.GetSiteAPIKey("another-site", first); e == nil {
		t.Fatal("cross site key read")
	}
	if e = s.DeleteSiteAPIKey(v.ID, second); e == nil {
		t.Fatal("deleted active key")
	}
	_, e = s.CreateSite(domain.CreateSiteInput{Name: "Rollback", BaseURL: "https://example.test", APIKey: "sk-x", ExtraAPIKeys: []domain.AddSiteApiKeyInput{{APIKey: "sk-x"}}})
	if e == nil {
		t.Fatal("accepted duplicate extra key")
	}
	sites, e := s.ListSites()
	if e != nil || len(sites) != 1 {
		t.Fatalf("create failed to rollback %d %v", len(sites), e)
	}
	old := models[0]
	e = s.ReplaceModels(v.ID, second, []domain.SiteModelDTO{{ModelID: "new"}, {ModelID: "bad", Raw: ptr(json.RawMessage("invalid"))}})
	if e == nil {
		t.Fatal("accepted invalid model JSON")
	}
	models, e = s.ListModels(v.ID)
	if e != nil || len(models) != 1 || models[0].ID != old.ID {
		t.Fatalf("replace failed to rollback %+v %v", models, e)
	}
	raw, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(raw), "sk-one-secret") || strings.Contains(string(raw), "apiKeyEncrypted") {
		t.Fatal("public DTO exposed secret")
	}
}
func TestModelExclusionsManualAndRoute(t *testing.T) {
	s := openTest(t)
	v := createTestSite(t, s)
	key := *v.ActiveAPIKeyID
	if e := s.SetSelectedModel(v.ID, key, "manual"); e != nil {
		t.Fatal(e)
	}
	fetched := []domain.SiteModelDTO{{ModelID: "a"}, {ModelID: "b"}}
	if e := s.ReplaceModels(v.ID, key, fetched); e != nil {
		t.Fatal(e)
	}
	if e := s.DeleteModel(v.ID, "a"); e != nil {
		t.Fatal(e)
	}
	if e := s.ReplaceModels(v.ID, key, fetched); e != nil {
		t.Fatal(e)
	}
	models, e := s.ListModels(v.ID)
	if e != nil || len(models) != 2 || models[0].ModelID != "b" || models[1].ModelID != "manual" {
		t.Fatalf("lost manual or exclusion %+v %v", models, e)
	}
	if e = s.SetSelectedModel(v.ID, key, "a"); e != nil {
		t.Fatal(e)
	}
	if e = s.ClearModels(v.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.ReplaceModels(v.ID, key, fetched); e != nil {
		t.Fatal(e)
	}
	models, e = s.ListModels(v.ID)
	if e != nil || len(models) != 2 {
		t.Fatalf("clear added exclusions %+v %v", models, e)
	}
	routes := []string{"https://a.test", "https://b.test"}
	v, e = s.UpdateSite(v.ID, domain.UpdateSiteInput{BaseURLs: &routes})
	if e != nil {
		t.Fatal(e)
	}
	v, e = s.SwitchSiteRoute(v.ID, "https://b.test")
	if e != nil || v.BaseURL != "https://b.test" || v.BaseURLs[1] != "https://a.test" {
		t.Fatalf("route order %+v %v", v, e)
	}
	if _, e = s.SwitchSiteRoute(v.ID, "https://unknown.test"); e == nil {
		t.Fatal("accepted unknown route")
	}
}
func TestWrongMissingKeyAndUnknownVersion(t *testing.T) {
	for _, mode := range []string{"missing", "wrong", "future"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			s, e := Open(root)
			if e != nil {
				t.Fatal(e)
			}
			createTestSite(t, s)
			if mode == "future" {
				if _, e = s.db.Exec("PRAGMA user_version=999"); e != nil {
					t.Fatal(e)
				}
			}
			if e = s.Close(); e != nil {
				t.Fatal(e)
			}
			keyPath := filepath.Join(root, "master.key")
			if mode == "missing" {
				e = os.Remove(keyPath)
			} else if mode == "wrong" {
				e = os.WriteFile(keyPath, make([]byte, 32), 0600)
			}
			if e != nil {
				t.Fatal(e)
			}
			if s, e = Open(root); e == nil {
				s.Close()
				t.Fatal("accepted unsafe state")
			}
			if mode == "missing" {
				if _, e = os.Stat(keyPath); !os.IsNotExist(e) {
					t.Fatal("recreated missing key")
				}
			}
			db, e := sql.Open("sqlite", filepath.Join(root, "xiaobai-switch.db"))
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			var n int
			if e = db.QueryRow("SELECT count(*) FROM sites").Scan(&n); e != nil || n != 1 {
				t.Fatalf("modified database %d %v", n, e)
			}
		})
	}
}
