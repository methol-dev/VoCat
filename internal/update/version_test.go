package update

import "testing"

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		current string
		latest  string
		want    bool
	}{
		{"0.0.3", "v0.0.4", true},
		{"0.0.4", "v0.0.4", false},
		{"0.1.0-dev", "v0.0.4", false},
		{"0.1.0-dev", "v0.1.0", true},
		{"1.2.3-rc.1", "v1.2.3-rc.2", true},
		{"1.2.3", "v1.2.3-rc.2", false},
		// Four-component releases: a revision supersedes the plain patch it
		// respins, and is in turn superseded by the next patch.
		{"0.1.14", "v0.1.14.1", true},
		{"0.1.14.1", "v0.1.14", false},
		{"0.1.14.1", "v0.1.14.1", false},
		{"0.1.14.1", "v0.1.14.2", true},
		{"0.1.14.2", "v0.1.14.1", false},
		{"0.1.14.1", "v0.1.15", true},
		{"0.1.15", "v0.1.14.1", false},
		{"0.1.14.1", "v0.2.0", true},
		{"0.1.13.9", "v0.1.14", true},
		// The revision composes with prerelease ordering the same way a patch does.
		{"0.1.14.1-rc.1", "v0.1.14.1", true},
		{"0.1.14.1", "v0.1.14.1-rc.2", false},
		{"0.1.14", "v0.1.14.1-rc.1", true},
	}
	for _, item := range tests {
		got, err := IsNewerVersion(item.current, item.latest)
		if err != nil {
			t.Errorf("IsNewerVersion(%q, %q): %v", item.current, item.latest, err)
			continue
		}
		if got != item.want {
			t.Errorf("IsNewerVersion(%q, %q) = %v, want %v", item.current, item.latest, got, item.want)
		}
	}
}

func TestIsNewerVersionRejectsInvalidRelease(t *testing.T) {
	if _, err := IsNewerVersion("0.1.0", "nightly"); err == nil {
		t.Fatal("invalid latest version was accepted")
	}
	// A fourth component widens the accepted shape by exactly one number; it
	// must not open the door to arbitrary dotted strings.
	for _, invalid := range []string{
		"0.1",
		"0.1.14.1.2",
		"0.1.14.",
		"0.1.14.x",
		"0.1.14.01",
		// ".0" is the three-component release spelled differently, not a respin.
		"0.1.14.0",
	} {
		if _, err := IsNewerVersion("0.1.0", invalid); err == nil {
			t.Errorf("invalid latest version %q was accepted", invalid)
		}
	}
}
