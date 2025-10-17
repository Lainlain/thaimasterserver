package version

import (
	"time"
)

// BuildInfo contains server build information
type BuildInfo struct {
	Version     string `json:"version"`
	BuildTime   string `json:"build_time"`
	GitCommit   string `json:"git_commit"`
	Features    []string `json:"features"`
}

// Current server version
var (
	Version   = "1.0.1"  // Increment this with each deployment
	BuildTime = time.Now().Format("2006-01-02 15:04:05")
	GitCommit = "affd419" // Latest commit with image API
)

// GetBuildInfo returns current build information
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
		Features: []string{
			"lottery_sse",
			"2d_history",
			"3d_results",
			"gifts_api",
			"sliders_api",
			"paper_api",
			"app_config_api",
			"admin_panel",
			"image_upload",
			"image_api_endpoint",  // NEW FEATURE
		},
	}
}
