package cli

import (
	"runtime/debug"
	"strings"
)

// Version is set for release binaries with -ldflags. Tagged module installs
// fall back to Go build information, while repository builds report dev.
var Version = "dev"

func currentVersion() string {
	info, ok := debug.ReadBuildInfo()
	return resolveVersion(Version, info, ok)
}

func resolveVersion(linkerVersion string, info *debug.BuildInfo, infoOK bool) string {
	if version := normalizeVersion(linkerVersion); version != "" && version != "dev" {
		return version
	}
	if infoOK && info != nil {
		if version := normalizeVersion(info.Main.Version); version != "" && version != "(devel)" {
			return version
		}
	}
	return "dev"
}

func normalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}
