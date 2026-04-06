package version

// Set via ldflags at build time:
//
//	go build -ldflags "-X github.com/blueberrycongee/wuu/internal/version.Version=v0.1.0"
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a human-readable version string.
func String() string {
	if Version == "dev" {
		return "wuu dev (built from source)"
	}
	return "wuu " + Version + " (" + Commit + " " + Date + ")"
}
