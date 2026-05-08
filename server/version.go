package server

import (
	"regexp"
	"strings"
)

const releaseNotesBaseURL = "https://github.com/jeeftor/openbooks/releases/tag/"

var releaseVersionPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

// VersionInfo describes the running build for UI display and release links.
type VersionInfo struct {
	DisplayVersion  string `json:"displayVersion"`
	RawVersion      string `json:"rawVersion"`
	CommitSHA       string `json:"commitSha"`
	BuildDate       string `json:"buildDate"`
	ReleaseNotesURL string `json:"releaseNotesUrl,omitempty"`
	IsRelease       bool   `json:"isRelease"`
}

// newVersionInfo builds the public version metadata returned by /version.
func newVersionInfo(version string, commitSHA string, buildDate string) VersionInfo {
	rawVersion := strings.TrimSpace(version)
	if rawVersion == "" {
		rawVersion = "unknown"
	}

	info := VersionInfo{
		DisplayVersion: devDisplayVersion(commitSHA),
		RawVersion:     rawVersion,
		CommitSHA:      normalizeBuildValue(commitSHA),
		BuildDate:      normalizeBuildValue(buildDate),
		IsRelease:      false,
	}

	if !releaseVersionPattern.MatchString(rawVersion) {
		return info
	}

	displayVersion := rawVersion
	if !strings.HasPrefix(displayVersion, "v") {
		displayVersion = "v" + displayVersion
	}

	info.DisplayVersion = displayVersion
	info.ReleaseNotesURL = releaseNotesBaseURL + displayVersion
	info.IsRelease = true
	return info
}

// devDisplayVersion returns a compact non-release label for UI display.
func devDisplayVersion(commitSHA string) string {
	shortSHA := shortCommitSHA(commitSHA)
	if shortSHA == "" {
		return "dev"
	}
	return "dev " + shortSHA
}

// shortCommitSHA returns the abbreviated commit used in dev build labels.
func shortCommitSHA(commitSHA string) string {
	commitSHA = normalizeBuildValue(commitSHA)
	if commitSHA == "" {
		return ""
	}
	if len(commitSHA) <= 7 {
		return commitSHA
	}
	return commitSHA[:7]
}

// normalizeBuildValue hides placeholder build values from API consumers.
func normalizeBuildValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "unknown" {
		return ""
	}
	return value
}
