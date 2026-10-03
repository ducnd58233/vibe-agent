package main

import "runtime/debug"

// version is stamped at build time with -ldflags "-X main.version=...":
// releases stamp their semver, and the Makefile stamps dev-<sha>. A plain
// `go build`, which is how both installers build from source, stamps nothing.
var version string

// currentVersion is the stamped version, or else the commit Go recorded in the
// binary, so a source build reports which commit it is rather than a
// placeholder. The installers used to stamp the literal "source", which told a
// person comparing installs nothing.
func currentVersion() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	return versionFromBuild(info, ok)
}

// versionFromBuild formats what Go recorded as dev-<sha>[-dirty].
func versionFromBuild(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil {
		return "dev"
	}
	revision, modified := "", false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision != "" {
		if len(revision) > 7 {
			revision = revision[:7]
		}
		if modified {
			return "dev-" + revision + "-dirty"
		}
		return "dev-" + revision
	}
	// `go install module@version` records the module version instead.
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	return "dev"
}
