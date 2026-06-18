package version

import (
	"runtime"
	"strings"
	"testing"
)

func TestWriteVersionToStream(t *testing.T) {
	Name = "venus"
	CommitID = "abc123"
	BuiltAt = "2026-06-14"
	Architecture = "amd64"
	Variant = "prod"
	RecentCommits = "feat: initial"

	buffer := new(strings.Builder)
	WriteVersionToStream(buffer)
	output := buffer.String()

	for _, want := range []string{
		"Name: venus",
		"CommitID: abc123",
		"BuiltAt: 2026-06-14",
		"Architecture: amd64",
		"Variant: prod",
		"RecentCommits: feat: initial",
		"GoVersion: " + runtime.Version(),
		"GoArchitecture: " + runtime.GOARCH,
	} {
		if !strings.Contains(output, want) {
			t.Errorf("version output missing %q, got:\n%s", want, output)
		}
	}
}
