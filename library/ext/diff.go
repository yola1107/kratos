package ext

import (
	"fmt"
	"strings"

	"github.com/r3labs/diff/v3"
)

// Diff returns a changelog of all mutated values from both
func Diff(a, b any) (diff.Changelog, error) {
	return diff.Diff(a, b)
}

func DiffLog(a, b any) (diff.Changelog, string, error) {
	changelog, err := diff.Diff(a, b)
	if err != nil {
		return nil, "", err
	}
	if len(changelog) == 0 {
		return nil, "", nil
	}

	// 提取字段名最大宽度
	maxKeyLen := 0
	maxFromLen := 0
	for _, change := range changelog {
		if field := fmt.Sprintf("%s", change.Path); len(field) > maxKeyLen {
			maxKeyLen = len(field)
		}
		if from := fmt.Sprintf("%v", change.From); len(from) > maxFromLen {
			maxFromLen = len(from)
		}
	}
	if maxFromLen > 8 {
		maxFromLen = 8
	}

	var sb strings.Builder
	for _, change := range changelog {
		field := fmt.Sprintf("%s", change.Path)
		sb.WriteString(fmt.Sprintf(
			"%-*s │ from: %-*v -> to: %v\n",
			maxKeyLen, field, maxFromLen, change.From, change.To,
		))
	}

	return changelog, sb.String(), nil
}

func DiffLog2(a, b any) (diff.Changelog, string, error) {
	changelog, err := diff.Diff(a, b)
	if err != nil {
		return nil, "", err
	}
	if len(changelog) == 0 {
		return nil, "", nil
	}
	fields := make([]string, len(changelog), len(changelog))
	for i, change := range changelog {
		fields[i] = fmt.Sprintf("Field=%s, From=%v, To=%v", change.Path, change.From, change.To)
	}
	return changelog, ToJSONPretty(fields), nil
}
