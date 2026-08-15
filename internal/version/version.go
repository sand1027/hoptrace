package version

// Version is injected at link time via:
//
//	go build -ldflags "-X github.com/sandeepv/hoptrace/internal/version.Version=v9.1.0"
var Version = "dev"
