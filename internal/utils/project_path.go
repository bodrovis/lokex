package utils

import (
	"fmt"
	"net/url"
)

// projectPath builds "projects/{id}/<suffix>" for project-scoped endpoints.
func ProjectPath(projectID, suffix string) string {
	return fmt.Sprintf("projects/%s/%s", url.PathEscape(projectID), suffix)
}
