package service

import (
	"encoding/json"
	"errors"
	"github.com/licoy/xiaobai-switch/internal/adapters"
	"github.com/licoy/xiaobai-switch/internal/domain"
	"github.com/licoy/xiaobai-switch/internal/envinject"
	"github.com/licoy/xiaobai-switch/internal/fileops"
	"github.com/licoy/xiaobai-switch/internal/paths"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type serviceJournal struct {
	Target           domain.TargetKind     `json:"target"`
	SiteID           string                `json:"siteId"`
	RecordID         string                `json:"recordId"`
	Operation        string                `json:"operation"`
	Phase            string                `json:"phase"`
	NoDB             bool                  `json:"noDb"`
	SiteNameSnapshot string                `json:"siteNameSnapshot"`
	ModelID          string                `json:"modelId"`
	BeforeBinding    *domain.TargetBinding `json:"beforeBinding"`
}

func saveServiceJournal(path string, j serviceJournal) error {
	raw, e := json.Marshal(j)
	if e != nil {
		return e
	}
	return fileops.AtomicWrite(path, raw)
}
func (s *Service) acquire(p adapters.Plan) (*fileops.Locks, error) {
	root, e := s.path("locks")
	if e != nil {
		return nil, e
	}
	keys := []string{string(p.Target)}
	if p.SiteID != "" {
		keys = append(keys, "site:"+p.SiteID)
	}
	cli := []string{}
	for _, c := range p.Changes {
		if _, e = s.validatePath(c.Path); e != nil {
			return nil, e
		}
		if p.Target == domain.Pi || p.Target == domain.Prime {
			if filepath.Base(c.Path) == "auth.json" || filepath.Base(c.Path) == "settings.json" {
				cli = append(cli, c.Path)
			}
		}
	}
	return fileops.Acquire(root, keys, cli)
}

// transact keeps target locks held by its caller across the file and DB commits.
func (s *Service) transact(p adapters.Plan, j serviceJournal, commit func(string) error) (string, error) {
	dir, e := s.path("backups", string(p.Target))
	if e != nil {
		return "", e
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	dir = filepath.Join(dir, strconv.FormatInt(time.Now().UnixNano(), 10)+"-"+domain.NewID())
	if dir, e = paths.ValidatePath(dir); e != nil {
		return "", e
	}
	j.Phase = "prepared"
	j.Target = p.Target
	j.SiteID = p.SiteID
	j.SiteNameSnapshot = p.Binding.SiteNameSnapshot
	j.ModelID = p.Binding.ModelID
	journalPath := dir + ".service.json"
	if e = s.writeJournal(journalPath, j); e != nil {
		return "", e
	}
	if len(p.Changes) > 0 {
		if _, e = adapters.ExecuteHeld(p, dir); e != nil {
			return dir, e
		}
	} else {
		if e = os.Mkdir(dir, 0700); e != nil {
			return dir, e
		}
	}
	if e = commit(dir); e != nil {
		rollbackErr := rollbackNative(dir)
		if rollbackErr == nil {
			j.Phase = "rolled_back"
			rollbackErr = s.writeJournal(journalPath, j)
		}
		return dir, errors.Join(e, rollbackErr)
	}
	j.Phase = "committed"
	if e = s.writeJournal(journalPath, j); e != nil {
		return dir, committedMaintenance("journal_finalize", p.Target, nil, e)
	}
	return dir, nil
}
func rollbackNative(dir string) error {
	_, e := os.Stat(filepath.Join(dir, "transaction.json"))
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	return envinject.Rollback(dir)
}
