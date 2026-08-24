package background

import "strings"

// normalizeProcessIDs trims inputs, preserves caller order and duplicates,
// and returns:
//   - ordered: trimmed IDs in original order, including empty IDs
//   - processMap: initial status per unique non-empty ID
//   - pending: set of unique non-empty IDs to poll
func normalizeProcessIDs(
	processIDs []string,
) ([]string, map[string]QueuedProcess, map[string]struct{}) {
	ordered := make([]string, len(processIDs))
	processMap := make(map[string]QueuedProcess, len(processIDs))
	pending := make(map[string]struct{}, len(processIDs))

	for i, raw := range processIDs {
		id := strings.TrimSpace(raw)
		ordered[i] = id

		if id == "" {
			continue
		}

		if _, ok := processMap[id]; !ok {
			processMap[id] = QueuedProcess{
				ProcessID: id,
				Status:    StatusQueued,
			}
		}

		pending[id] = struct{}{}
	}

	return ordered, processMap, pending
}
