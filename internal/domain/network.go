package domain

type URLWritePreview struct {
	ModelsURL     string `json:"modelsUrl"`
	ClaudeBaseURL string `json:"claudeBaseUrl"`
	CodexBaseURL  string `json:"codexBaseUrl"`
}
type HTTPTextResult struct {
	Status      uint16 `json:"status"`
	ContentType string `json:"contentType"`
	FinalURL    string `json:"finalUrl"`
	Body        string `json:"body"`
}
