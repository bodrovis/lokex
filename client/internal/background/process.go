package background

import (
	"strings"

	"github.com/bodrovis/lokex/v2/internal/utils"
)

// QueuedProcess is a normalized view over Lokalise "processes/*" responses.
// DownloadURL is populated when the process produces a file (e.g., download).
type QueuedProcess struct {
	ProcessID   string `json:"process_id"`
	Status      string `json:"status"`
	DownloadURL string `json:"download_url,omitempty"`
	Message     string `json:"message,omitempty"`
}

// processResponse mirrors the subset of the Lokalise response we care about.
// It stays unexported; callers use QueuedProcess instead.
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

// ToQueuedProcess converts a typed API response into a flattened QueuedProcess.
func (pr *processResponse) ToQueuedProcess() QueuedProcess {
	return QueuedProcess{
		ProcessID:   pr.Process.ProcessID,
		Status:      utils.NormalizeString(pr.Process.Status),
		Message:     strings.TrimSpace(pr.Process.Message),
		DownloadURL: pr.Process.Details.DownloadURL,
	}
}
