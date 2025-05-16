package function

import (
	"fmt"

	"github.com/r3labs/diff/v3"
)

// Diff returns a changelog of all mutated values from both
func Diff(a, b any) (diff.Changelog, error) {
	return diff.Diff(a, b)
}

func DiffLog(a, b any) (diff.Changelog, []string, error) {
	changelog, _ := diff.Diff(a, b)
	fields := make([]string, 0, len(changelog))
	for _, change := range changelog {
		fields = append(fields, fmt.Sprintf("Field=%s, From=%v, To=%v", change.Path, change.From, change.To))
	}
	return changelog, fields, nil
}
