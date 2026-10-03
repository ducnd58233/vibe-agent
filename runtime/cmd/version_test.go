package main

import (
	"runtime/debug"
	"testing"
)

func TestVersionFromBuildReportsTheRecordedCommit(t *testing.T) {
	info := func(main string, settings ...debug.BuildSetting) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Version: main}, Settings: settings}
	}
	rev := debug.BuildSetting{Key: "vcs.revision", Value: "87283c13b39b465ab19eca176f3694c4498030ad"}
	cases := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"clean source build", info("(devel)", rev, debug.BuildSetting{Key: "vcs.modified", Value: "false"}), true, "dev-87283c1"},
		{"dirty source build", info("(devel)", rev, debug.BuildSetting{Key: "vcs.modified", Value: "true"}), true, "dev-87283c1-dirty"},
		{"go install at a version", info("v0.2.0"), true, "v0.2.0"},
		{"no VCS information", info("(devel)"), true, "dev"},
		{"no build information", nil, false, "dev"},
	}
	for _, tc := range cases {
		if got := versionFromBuild(tc.info, tc.ok); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestAStampedVersionWins(t *testing.T) {
	saved := version
	t.Cleanup(func() { version = saved })
	version = "0.1.7-dev.87283c1"
	if got := currentVersion(); got != version {
		t.Errorf("currentVersion() = %q, want the stamped %q", got, version)
	}
}
