package background

import (
	"strings"
)

// QueuedProcess is a normalized view of a Lokalise process response.
// DownloadURL is populated when the process produces a file.
type QueuedProcess struct {
	ProcessID   string `json:"process_id"`
	Status      string `json:"status"`
	DownloadURL string `json:"download_url,omitempty"`
	Message     string `json:"message,omitempty"`
}

// processResponse mirrors the subset of the Lokalise response we care about.
type processResponse struct {
	Process struct {
		ProcessID string `json:"process_id"`
		Status    string `json:"status"`
		Message   string `json:"message"`
		Details   struct {
			DownloadURL string `json:"download_url"`
		} `json:"details"`
	} `json:"process"`
}

// ToQueuedProcess converts the API response into a flattened QueuedProcess.
func (pr *processResponse) ToQueuedProcess() QueuedProcess {
	return QueuedProcess{
		ProcessID:   pr.Process.ProcessID,
		Status:      strings.ToLower(strings.TrimSpace(pr.Process.Status)),
		Message:     strings.TrimSpace(pr.Process.Message),
		DownloadURL: pr.Process.Details.DownloadURL,
	}
}
