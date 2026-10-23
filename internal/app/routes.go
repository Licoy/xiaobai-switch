package app

type Route struct {
	ID, Label, Icon, Group string
	Tint                   Tint
}

var routes = []Route{
	{"overview", "overview", "dashboard-line", "", TintBlue}, {"sites", "sites", "apps-line", "", TintIndigo}, {"skills", "skills", "puzzle-line", "", TintYellow},
	{"claude", "Claude Code", "ClaudeCode", "apply", TintOrange}, {"codex", "Codex", "Codex", "apply", TintGray}, {"pi", "Pi", "Pi", "apply", TintTeal}, {"prime", "Prime", "sparkling-line", "apply", TintPurple},
	{"appearance", "appearance", "palette-line", "general", TintPink}, {"network", "network", "global-line", "general", TintTeal}, {"paths", "paths", "folder-settings-line", "general", TintOrange}, {"backups", "backups", "history-line", "general", TintGreen}, {"about", "about", "information-line", "general", TintBlue},
}

func routeByID(id string) Route {
	for _, r := range routes {
		if r.ID == id {
			return r
		}
	}
	return routes[0]
}
