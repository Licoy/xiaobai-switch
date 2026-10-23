package l10n

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/licoy/xiaobai-switch/locales"
)

type Catalog map[string]string

func Load(language string) (Catalog, error) {
	if strings.HasPrefix(strings.ToLower(language), "en") {
		language = "en-US"
	} else {
		language = "zh-CN"
	}
	data, err := locales.Files.ReadFile(language + ".json")
	if err != nil {
		return nil, err
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}
	return catalog, nil
}

func (c Catalog) Text(key string) string {
	text, ok := c[key]
	if !ok {
		panic(fmt.Sprintf("missing translation: %s", key))
	}
	return text
}
