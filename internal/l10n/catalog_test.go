package l10n

import "testing"

func TestCatalogParity(t *testing.T) {
	zh, err := Load("zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	en, err := Load("en-US")
	if err != nil {
		t.Fatal(err)
	}
	if len(zh) != len(en) {
		t.Fatalf("translation key counts differ: %d/%d", len(zh), len(en))
	}
	for key, value := range zh {
		if value == "" || en[key] == "" {
			t.Errorf("missing translation %s", key)
		}
	}
}
