package in

import "github.com/samcontesse/gitlab-merge-request-resource/pkg"

type Request struct {
	Source  pkg.Source  `json:"source"`
	Version pkg.Version `json:"version"`
}

type Response struct {
	Version  pkg.Version  `json:"version"`
	Metadata pkg.Metadata `json:"metadata"`
}
