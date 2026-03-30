package background

// buildResults reconstructs output preserving caller order and duplicates.
// Empty IDs are skipped.
func buildResults(ordered []string, processMap map[string]QueuedProcess) []QueuedProcess {
	out := make([]QueuedProcess, 0, len(ordered))
	for _, id := range ordered {
		if id == "" {
			continue
		}
		if p, ok := processMap[id]; ok {
			out = append(out, p)
		} else {
			out = append(out, QueuedProcess{ProcessID: id, Status: StatusQueued})
		}
	}
	return out
}
