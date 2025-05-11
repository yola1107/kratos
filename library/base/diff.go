package base

import (
	"github.com/r3labs/diff/v3"
)

// Diff returns a changelog of all mutated values from both
func Diff(a, b interface{}) (diff.Changelog, error) {
	return diff.Diff(a, b)
}
