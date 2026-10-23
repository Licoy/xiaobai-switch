package skills

import (
	"context"
	"encoding/json"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/netx"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type installedIndex struct {
	repo  map[string][]domain.SkillTarget
	skill map[string][]domain.SkillTarget
}

func (m *Manager) installed() (installedIndex, error) {
	index := installedIndex{repo: map[string][]domain.SkillTarget{}, skill: map[string][]domain.SkillTarget{}}
	for _, target := range targets {
		root, e := m.root(target)
		if e != nil {
			return index, e
		}
		if e = collectInstalled(root, target, 0, &index); e != nil {
			return index, e
		}
	}
	return index, nil
}
func collectInstalled(dir string, target domain.SkillTarget, depth int, index *installedIndex) error {
	if depth > 12 {
		return validation("skill nesting exceeds limit")
	}
	p := filepath.Join(dir, installManifest)
	if _, e := os.Lstat(p); e == nil {
		manifest, e := readInstallation(p)
		if e != nil {
			return e
		}
		ref := normalizeSource(manifest.SourceRef)
		index.repo[ref] = append(index.repo[ref], target)
		if manifest.SkillName != nil {
			key := ref + "\x00" + strings.ToLower(*manifest.SkillName)
			index.skill[key] = append(index.skill[key], target)
		}
		return nil
	} else if !os.IsNotExist(e) {
		return e
	}
	entries, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Type()&os.ModeSymlink == 0 && !strings.HasPrefix(entry.Name(), ".") {
			if e = collectInstalled(filepath.Join(dir, entry.Name()), target, depth+1, index); e != nil {
				return e
			}
		}
	}
	return nil
}
func (i installedIndex) targets(ref, name string, bySkill bool) []domain.SkillTarget {
	ref = normalizeSource(ref)
	values := i.repo[ref]
	if bySkill {
		key := ref + "\x00" + strings.ToLower(name)
		if found, ok := i.skill[key]; ok {
			values = found
		} else {
			for key := range i.skill {
				if strings.HasPrefix(key, ref+"\x00") {
					values = nil
					break
				}
			}
		}
	}
	out := []domain.SkillTarget{}
	for _, target := range targets {
		for _, v := range values {
			if v == target {
				out = append(out, target)
				break
			}
		}
	}
	return out
}
func (m *Manager) Search(ctx context.Context, query, source string) ([]domain.MarketplaceSkill, error) {
	query = strings.TrimSpace(query)
	endpoint := "https://skills.sh/api/search"
	params := url.Values{"limit": {"20"}}
	bySkill := true
	switch source {
	case "", "skills.sh":
		if query == "" {
			query = "skill"
		} else if utf8.RuneCountInString(query) < 2 {
			return nil, validation("skills.sh search requires at least two characters")
		}
		params.Set("q", query)
	case "github":
		endpoint = "https://api.github.com/search/repositories"
		if query == "" {
			query = "topic:agent-skill"
		} else {
			query += " topic:agent-skill"
		}
		params = url.Values{"q": {query}, "sort": {"stars"}, "per_page": {"20"}}
		bySkill = false
	default:
		return nil, validation("unsupported marketplace source")
	}
	fsMu.Lock()
	index, e := m.installed()
	fsMu.Unlock()
	if e != nil {
		return nil, e
	}
	req, e := netx.Request(ctx, m.client, endpoint+"?"+params.Encode())
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, e := m.client.Do(req)
	if e != nil {
		return nil, netx.NetworkError(e, "")
	}
	raw, e := netx.ReadBody(resp, 4*1024*1024)
	if e != nil {
		return nil, e
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, netx.StatusError(resp.StatusCode, raw, "")
	}
	var body map[string]json.RawMessage
	if e = json.Unmarshal(raw, &body); e != nil {
		return nil, e
	}
	key := "skills"
	if !bySkill {
		key = "items"
	}
	var items []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Source      string `json:"source"`
		FullName    string `json:"full_name"`
		Stars       int64  `json:"stars"`
		GitHubStars int64  `json:"stargazers_count"`
		Installs    int64  `json:"installs"`
	}
	if value := body[key]; len(value) == 0 || string(value) == "null" || json.Unmarshal(value, &items) != nil {
		return nil, domain.NewError("invalid_response", "invalid marketplace response")
	}
	out := []domain.MarketplaceSkill{}
	for _, item := range items {
		repo := item.Source
		stars := item.Stars
		installs := item.Installs
		if !bySkill {
			repo = item.FullName
			stars = item.GitHubStars
			installs = 0
		}
		if strings.TrimSpace(item.Name) == "" || normalizeSource(repo) == "" {
			continue
		}
		out = append(out, domain.MarketplaceSkill{Name: item.Name, Description: item.Description, Repo: repo, Stars: stars, Installs: installs, InstalledTargets: index.targets(repo, item.Name, bySkill)})
		if len(out) == 20 {
			break
		}
	}
	return out, nil
}
