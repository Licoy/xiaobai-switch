package adapters

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/licoy/xiaobai-switch/internal/domain"
)

func TestOfficialRestoreAfterAgentReformatsConfig(t *testing.T) {
	for _, target := range []domain.TargetKind{domain.Pi, domain.Prime} {
		t.Run(string(target), func(t *testing.T) {
			r := request(t, target)
			fixture(t, r.Home, "settings.json", "{\n  \"defaultProvider\": \"xiaobai_test\",\n  \"defaultModel\": \"test-model\",\n  \"theme\": \"dark\"\n}\n")
			restore, err := RestoreOfficial(RestoreRequest{Target: r.Target, Home: r.Home})
			if err != nil {
				t.Fatal(err)
			}
			for _, change := range restore.Changes {
				live, err := os.ReadFile(change.Path)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(change.Before, live) {
					t.Fatal("restore plan changed original snapshot before execution")
				}
			}
			apply(t, restore)
			settings := jsonFile(t, filepath.Join(r.Home, "settings.json"))
			if len(settings) != 1 || settings["theme"] != "dark" {
				t.Fatal("restore must remove managed defaults and preserve user settings")
			}
		})
	}
}

func TestCodexOfficialRestoreWithoutBinding(t *testing.T) {
	for index, body := range []string{
		"model = \"managed-model\"\nmodel_provider = \"xiaobai_fixture\"\n[model_providers.xiaobai_fixture]\nname = \"fixture\"\nenv_key = \"XIAOBAI_FIXTURE_API_KEY\"\n[model_providers.other]\nname = \"keep\" # keep comment\n",
		"model = \"managed-model\"\nmodel_provider = \"xiaobai_fixture\"\nmodel_providers = { xiaobai_fixture = { name = \"fixture\", env_key = \"XIAOBAI_FIXTURE_API_KEY\" }, other = { name = \"keep\" } } # keep comment\n",
	} {
		t.Run([]string{"section", "inline"}[index], func(t *testing.T) {
			r := request(t, domain.Codex)
			fixture(t, r.Home, "config.toml", body)
			fixture(t, r.Home, "codex.env", "export XIAOBAI_FIXTURE_API_KEY='fixture-secret'\nexport OTHER='keep'\n")
			restore, err := RestoreOfficial(RestoreRequest{Target: r.Target, Home: r.Home, EnvPath: r.EnvPath})
			if err != nil {
				t.Fatal(err)
			}
			apply(t, restore)
			raw, err := os.ReadFile(filepath.Join(r.Home, "config.toml"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "xiaobai_fixture") || !strings.Contains(string(raw), "# keep comment") || !strings.Contains(string(raw), "keep") {
				t.Fatalf("restore retained managed provider or lost unrelated config: %s", raw)
			}
			raw, err = os.ReadFile(r.EnvPath)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "XIAOBAI_FIXTURE_API_KEY") || !strings.Contains(string(raw), "OTHER='keep'") {
				t.Fatal("restore retained managed key or lost unrelated env")
			}
		})
	}
}

func TestCodexOfficialRestorePlansWindowsEnvironmentRemoval(t *testing.T) {
	r := request(t, domain.Codex)
	r.Platform, r.EnvMode = "windows", "user_env"
	built, err := Build(r)
	if err != nil {
		t.Fatal(err)
	}
	// Build only: this test never executes or reads the Windows user environment.
	restore, err := RestoreOfficial(RestoreRequest{Target: r.Target, Home: r.Home, EnvPath: r.EnvPath, Binding: &built.Binding})
	if err != nil {
		t.Fatal(err)
	}
	injection := restore.Injection
	if injection == nil || injection.Mode != "user_env" || !injection.UserRemove || injection.UserKey != domain.EnvKeyForSite(r.Site.ID) || injection.UserExpectedFingerprint != built.Binding.KeyFingerprint {
		t.Fatal("official restore omitted guarded Windows user environment removal")
	}
}

func TestCodexOfficialRestoreRemovesLegacyBaseURL(t *testing.T) {
	r := request(t, domain.Codex)
	fixture(t, r.Home, "config.toml", "openai_base_url = \"https://relay.example.test/v1\"\n[mcp_servers.keep]\ncommand = \"fixture\" # keep comment\n")
	restore, err := RestoreOfficial(RestoreRequest{Target: r.Target, Home: r.Home, EnvPath: r.EnvPath})
	if err != nil {
		t.Fatal(err)
	}
	apply(t, restore)
	raw, err := os.ReadFile(filepath.Join(r.Home, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "openai_base_url") {
		t.Fatal("official restore retained legacy relay base URL")
	}
	if !strings.Contains(string(raw), "# keep comment") || !strings.Contains(string(raw), "mcp_servers.keep") {
		t.Fatal("official restore changed unrelated MCP config")
	}
}

func TestOfficialRestoreDoesNotChangeInputBinding(t *testing.T) {
	for _, target := range []domain.TargetKind{domain.ClaudeCode, domain.Codex, domain.Pi, domain.Prime} {
		t.Run(string(target), func(t *testing.T) {
			r := request(t, target)
			built, err := Build(r)
			if err != nil {
				t.Fatal(err)
			}
			apply(t, built)
			before, err := json.Marshal(built.Binding)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = RestoreOfficial(RestoreRequest{Target: r.Target, Home: r.Home, EnvPath: r.EnvPath, Binding: &built.Binding}); err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(built.Binding)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("official restore mutated caller binding")
			}
		})
	}
}
