package skills

import (
	"gopkg.in/yaml.v3"
	"strings"
)

type parsedSkill struct {
	name, description string
	author, version   *string
}

func scalar(node yaml.Node) *string {
	if node.Kind != yaml.ScalarNode || node.Tag == "!!null" {
		return nil
	}
	s := node.Value
	return &s
}
func parseSkill(content string) (parsedSkill, error) {
	lines := strings.SplitAfter(content, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r\n") != "---" {
		return parsedSkill{}, validation("skill must begin with YAML frontmatter")
	}
	var block strings.Builder
	closed := false
	for _, line := range lines[1:] {
		if strings.TrimRight(line, "\r\n") == "---" {
			closed = true
			break
		}
		block.WriteString(line)
	}
	if !closed {
		return parsedSkill{}, validation("skill frontmatter is not closed")
	}
	var front struct {
		Name        string               `yaml:"name"`
		Description string               `yaml:"description"`
		Author      yaml.Node            `yaml:"author"`
		Version     yaml.Node            `yaml:"version"`
		Metadata    map[string]yaml.Node `yaml:"metadata"`
	}
	if e := yaml.Unmarshal([]byte(block.String()), &front); e != nil {
		return parsedSkill{}, validation("invalid YAML skill frontmatter: " + e.Error())
	}
	p := parsedSkill{name: strings.TrimSpace(front.Name), description: strings.TrimSpace(front.Description), author: scalar(front.Author), version: scalar(front.Version)}
	if p.name == "" || p.description == "" {
		return p, validation("skill frontmatter requires name and description")
	}
	if p.author == nil {
		p.author = scalar(front.Metadata["author"])
	}
	if p.version == nil {
		p.version = scalar(front.Metadata["version"])
	}
	return p, nil
}
