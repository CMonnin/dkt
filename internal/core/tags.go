package core

import (
	"slices"
	"strings"
)

// NormalizeTag strips a leading '#' and lowercases.
func NormalizeTag(tag string) string {
	return strings.ToLower(strings.TrimLeft(strings.TrimSpace(tag), "#"))
}

// ParseTitle pulls inline "#tag" words out of a title.
func ParseTitle(s string) (title string, tags []string) {
	var words []string
	for _, f := range strings.Fields(s) {
		if len(f) > 1 && f[0] == '#' {
			if tag := NormalizeTag(f); tag != "" && !slices.Contains(tags, tag) {
				tags = append(tags, tag)
			}
			continue
		}
		words = append(words, f)
	}
	return strings.Join(words, " "), tags
}

// SplitTags parses "a,b c" into normalized tags.
func SplitTags(s string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		if tag := NormalizeTag(f); tag != "" && !slices.Contains(out, tag) {
			out = append(out, tag)
		}
	}
	return out
}
