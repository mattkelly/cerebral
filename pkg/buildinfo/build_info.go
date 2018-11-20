package buildinfo

import (
	"fmt"
	"strconv"
	"time"
)

// These variables should be initialized by ldflags
// Times should be the string representation of a Unix (epoch) time, i.e.
// the output of `date '+%s'`.
var (
	gitDescribe string
	gitCommit   string
	unixTime    string
)

// Spec represents the build info for the running binary
type Spec struct {
	// GitDescribe is the git describe for this build
	// It should be a clean git tag for a real release
	GitDescribe string `json:"git_describe"`
	// GitCommit is the git commit for this build
	GitCommit string `json:"git_commit"`
	// Timestamp is the Unix (epoch) time when the build occurred
	Timestamp time.Time `json:"timestamp"`
}

var buildInfo Spec

// Initialize the buildInfo struct. Strings that were not properly initialized
// by ldflags will be set to a dummy value. Times that were not properly initialized
// will use their zero value.
func init() {
	buildInfo.GitDescribe = gitDescribe
	if buildInfo.GitDescribe == "" {
		buildInfo.GitDescribe = "unknown"
	}

	buildInfo.GitCommit = gitCommit
	if buildInfo.GitCommit == "" {
		buildInfo.GitCommit = "unknown"
	}

	secs, err := strconv.ParseInt(unixTime, 10, 64)
	if err != nil {
		secs = 0
	}
	buildInfo.Timestamp = time.Unix(secs, 0)
}

// Get returns a copy of the build info
func Get() Spec {
	return buildInfo
}

// String returns a string representation of the build info
func String() string {
	return fmt.Sprintf("%s (%s) built @ %v",
		buildInfo.GitDescribe, buildInfo.GitCommit, buildInfo.Timestamp)
}
