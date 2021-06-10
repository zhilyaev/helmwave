package plan

import "github.com/r3labs/diff/v2"

func Diff(a *Plan, b *Plan) (diff.Changelog, error) {
	return diff.Diff(a, b, diff.AllowTypeMismatch(true))
}