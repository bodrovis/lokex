package utils_test

import (
	"testing"

	"github.com/bodrovis/lokex/v2/internal/utils"
)

func TestProjectPath(t *testing.T) {
	tests := []struct {
		name      string
		projectID string
		suffix    string
		want      string
	}{
		{
			name:      "basic project path",
			projectID: "123.abc",
			suffix:    "files/download",
			want:      "projects/123.abc/files/download",
		},
		{
			name:      "escapes project ID",
			projectID: "project/with spaces",
			suffix:    "files/upload",
			want:      "projects/project%2Fwith%20spaces/files/upload",
		},
		{
			name:      "empty suffix",
			projectID: "123",
			suffix:    "",
			want:      "projects/123/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.ProjectPath(tt.projectID, tt.suffix)

			if got != tt.want {
				t.Fatalf("ProjectPath(%q, %q) = %q, want %q",
					tt.projectID, tt.suffix, got, tt.want)
			}
		})
	}
}
