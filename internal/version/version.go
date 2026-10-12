package version

// Version is overridden at release time:
// -ldflags "-X github.com/kushiemoon-dev/quaver-stream-companion/internal/version.Version=v0.1.0"
var Version = "dev"

func String() string {
	return "quaver-stream-companion " + Version
}
