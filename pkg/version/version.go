package version

import (
	"io"
	"runtime"
	"text/template"
)

var (
	// Name is the application name, should be set by consumers via ldflags or init
	Name string

	// CommitID is the git commit hash
	CommitID string
	// BuiltAt is the build timestamp
	BuiltAt string
	// Architecture is the target architecture
	Architecture string
	// Variant is the build variant
	Variant string
	// RecentCommits contains recent commit history
	RecentCommits string

	versionTemplate = template.Must(template.New("version").Parse(`Name: {{ .Name }}
CommitID: {{ .CommitID }}
BuiltAt: {{ .BuiltAt }}
Architecture: {{ .Architecture }}
GoVersion: {{ .GoVersion }}
GoArchitecture: {{ .GoArchitecture }}
Variant: {{ .Variant }}
----------
RecentCommits: {{ .RecentCommits }}
`))
)

// WriteVersionToStream writes version information to the provided writer
func WriteVersionToStream(buffer io.Writer) {
	_ = versionTemplate.Execute(buffer, &struct {
		Name           string
		CommitID       string
		BuiltAt        string
		Architecture   string
		Variant        string
		RecentCommits  string
		GoVersion      string
		GoArchitecture string
	}{
		Name:           Name,
		CommitID:       CommitID,
		BuiltAt:        BuiltAt,
		Architecture:   Architecture,
		Variant:        Variant,
		RecentCommits:  RecentCommits,
		GoVersion:      runtime.Version(),
		GoArchitecture: runtime.GOARCH,
	})
}
