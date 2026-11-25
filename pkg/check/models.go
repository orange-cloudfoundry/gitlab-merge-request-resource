package check

import "github.com/samcontesse/gitlab-merge-request-resource/pkg"

type Request struct {
	Source  pkg.Source  `json:"source"`
	Version pkg.Version `json:"version,omitempty"`
}

type Response []pkg.Version
