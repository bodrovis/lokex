package background

import (
	"strings"
)

// normalizeProcessIDs trims inputs, preserves caller order (including duplicates),
// and returns:
//   - ordered: trimmed IDs in original order (empties kept as "")
//   - processMap: latest status per UNIQUE non-empty ID (seeded with StatusQueued)
//   - pending: set of UNIQUE non-empty IDs to poll
func normalizeProcessIDs(processIDs []string) (ordered []string, processMap map[string]QueuedProcess, pending map[string]struct{}) {
	ordered = make([]string, 0, len(processIDs))
	processMap = make(map[string]QueuedProcess, len(processIDs))
	pending = make(map[string]struct{}, len(processIDs))

	for _, raw := range processIDs {
		id := strings.TrimSpace(raw)
		ordered = append(ordered, id)
		if id == "" {
			continue
		}
		if _, ok := processMap[id]; !ok {
			processMap[id] = QueuedProcess{ProcessID: id, Status: StatusQueued}
		}
		pending[id] = struct{}{}
	}
	return
}
