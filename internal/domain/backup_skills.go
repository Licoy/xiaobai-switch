package domain

type RestoreQueueResult struct {
	Warning          *string `json:"warning,omitempty"`
	RelaunchRequired bool    `json:"relaunchRequired"`
	FileName         string  `json:"fileName"`
}
type SkillTarget string

const (
	SkillAgents     SkillTarget = "agents"
	SkillClaudeCode SkillTarget = "claude_code"
	SkillCodex      SkillTarget = "codex"
	SkillPi         SkillTarget = "pi"
	SkillPrime      SkillTarget = "prime"
)

func (t SkillTarget) Valid() bool {
	return t == SkillAgents || t == SkillClaudeCode || t == SkillCodex || t == SkillPi || t == SkillPrime
}

type SkillInfo struct {
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	Author        *string     `json:"author"`
	Version       *string     `json:"version"`
	Target        SkillTarget `json:"target"`
	SourcePath    string      `json:"sourcePath"`
	SkillsPath    string      `json:"skillsPath"`
	DirectoryPath string      `json:"directoryPath"`
	Enabled       bool        `json:"enabled"`
}
type SkillDetail struct {
	Info    SkillInfo `json:"info"`
	Content string    `json:"content"`
	Files   []string  `json:"files"`
}
type MarketplaceSkill struct {
	Name             string        `json:"name"`
	Description      string        `json:"description"`
	Repo             string        `json:"repo"`
	Stars            int64         `json:"stars"`
	Installs         int64         `json:"installs"`
	InstalledTargets []SkillTarget `json:"installedTargets"`
}
type SkillInstallResult struct {
	Names    []string `json:"names"`
	Paths    []string `json:"paths"`
	Warnings []string `json:"warnings"`
}
