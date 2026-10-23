package siteicon

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"

	"github.com/licoy/xiaobai-switch/internal/fileops"
)

var safeID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// Cache keeps the icon found for each site and the origin it came from, a site without one included
type Cache struct{ Dir string }

const cacheVersion = 1

type cached struct {
	Version int    `json:"version"`
	Origin  string `json:"origin"`
	Icon    []byte `json:"icon"`
}

func (c Cache) path(siteID string) (string, bool) {
	if c.Dir == "" || !safeID.MatchString(siteID) {
		return "", false
	}
	return filepath.Join(c.Dir, siteID+".json"), true
}

// Load reports the icon cached for the site at origin; hit is false when there is none for that origin
func (c Cache) Load(siteID, origin string) (icon []byte, hit bool) {
	path, ok := c.path(siteID)
	if !ok {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var v cached
	// Refresh old misses once so earlier fetch failures do not hide newly available icons
	if json.Unmarshal(data, &v) != nil || v.Origin != origin || (v.Version != cacheVersion && len(v.Icon) == 0) {
		return nil, false
	}
	return v.Icon, true
}

// Save records the icon of the site at origin, nil for none
func (c Cache) Save(siteID, origin string, icon []byte) error {
	path, ok := c.path(siteID)
	if !ok {
		return nil
	}
	data, err := json.Marshal(cached{Version: cacheVersion, Origin: origin, Icon: icon})
	if err != nil {
		return err
	}
	return fileops.AtomicWrite(path, data)
}

// Invalidate forgets the icon of the site
func (c Cache) Invalidate(siteID string) error {
	path, ok := c.path(siteID)
	if !ok {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
