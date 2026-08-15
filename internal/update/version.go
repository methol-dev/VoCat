package update

import (
	"fmt"
	"strconv"
	"strings"
)

type semanticVersion struct {
	major int
	minor int
	patch int
	// revision is the optional fourth component of a MAJOR.MINOR.PATCH.REVISION
	// release such as 0.1.14.1, used for a respin of an existing patch release.
	// A three-component version has revision 0, which orders it before any
	// revision of the same patch: 0.1.14 < 0.1.14.1 < 0.1.15.
	revision   int
	prerelease string
}

// IsNewerVersion reports whether latest is newer than current. Both values
// may include the conventional v prefix, an optional fourth revision
// component, prerelease suffixes, and build metadata. Invalid release versions
// are rejected instead of triggering a downgrade or an arbitrary file
// replacement.
func IsNewerVersion(current, latest string) (bool, error) {
	currentVersion, err := parseSemanticVersion(current)
	if err != nil {
		return false, fmt.Errorf("current version: %w", err)
	}
	latestVersion, err := parseSemanticVersion(latest)
	if err != nil {
		return false, fmt.Errorf("latest version: %w", err)
	}
	if currentVersion.major != latestVersion.major {
		return latestVersion.major > currentVersion.major, nil
	}
	if currentVersion.minor != latestVersion.minor {
		return latestVersion.minor > currentVersion.minor, nil
	}
	if currentVersion.patch != latestVersion.patch {
		return latestVersion.patch > currentVersion.patch, nil
	}
	if currentVersion.revision != latestVersion.revision {
		return latestVersion.revision > currentVersion.revision, nil
	}
	if currentVersion.prerelease == latestVersion.prerelease {
		return false, nil
	}
	if currentVersion.prerelease != "" && latestVersion.prerelease == "" {
		return true, nil
	}
	if currentVersion.prerelease == "" {
		return false, nil
	}
	return comparePrerelease(currentVersion.prerelease, latestVersion.prerelease) < 0, nil
}

func parseSemanticVersion(raw string) (semanticVersion, error) {
	value := strings.TrimPrefix(strings.TrimSpace(raw), "v")
	if build := strings.IndexByte(value, '+'); build >= 0 {
		value = value[:build]
	}
	prerelease := ""
	hasPrerelease := false
	if dash := strings.IndexByte(value, '-'); dash >= 0 {
		hasPrerelease = true
		prerelease = value[dash+1:]
		value = value[:dash]
	}
	parts := strings.Split(value, ".")
	// Three components is plain semver; a fourth is the optional revision of a
	// respun patch release. Anything else is rejected.
	if len(parts) < 3 || len(parts) > 4 || (hasPrerelease && prerelease == "") {
		return semanticVersion{}, fmt.Errorf("%q is not a semantic version", raw)
	}
	numbers := make([]int, 4)
	for index, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return semanticVersion{}, fmt.Errorf("%q is not a semantic version", raw)
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return semanticVersion{}, fmt.Errorf("%q is not a semantic version", raw)
		}
		numbers[index] = value
	}
	// A ".0" revision is the same release as the three-component form, so it is
	// rejected rather than silently accepted as a second spelling of it.
	if len(parts) == 4 && numbers[3] == 0 {
		return semanticVersion{}, fmt.Errorf("%q is not a semantic version", raw)
	}
	if strings.ContainsAny(prerelease, " \t\r\n") {
		return semanticVersion{}, fmt.Errorf("%q is not a semantic version", raw)
	}
	return semanticVersion{
		major:      numbers[0],
		minor:      numbers[1],
		patch:      numbers[2],
		revision:   numbers[3],
		prerelease: prerelease,
	}, nil
}

func comparePrerelease(left, right string) int {
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
		if leftParts[index] == rightParts[index] {
			continue
		}
		leftNumber, leftErr := strconv.Atoi(leftParts[index])
		rightNumber, rightErr := strconv.Atoi(rightParts[index])
		switch {
		case leftErr == nil && rightErr == nil:
			if leftNumber < rightNumber {
				return -1
			}
			return 1
		case leftErr == nil:
			return -1
		case rightErr == nil:
			return 1
		case leftParts[index] < rightParts[index]:
			return -1
		default:
			return 1
		}
	}
	if len(leftParts) < len(rightParts) {
		return -1
	}
	if len(leftParts) > len(rightParts) {
		return 1
	}
	return 0
}
