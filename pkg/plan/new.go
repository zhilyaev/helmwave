package plan

import (
	"github.com/helmwave/helmwave/pkg/release"
	"github.com/helmwave/helmwave/pkg/repo"
)

type Plan struct {
	dir      string
	fullPath string
	body     *planBody
}

const planfile = "planfile"

type planBody struct {
	Project      string
	Version      string
	Repositories []*repo.Config
	Releases     []*release.Config
}

func New(dir string) *Plan {
	plan := &Plan{
		dir:      dir,
		fullPath: dir + planfile,
	}

	return plan
}