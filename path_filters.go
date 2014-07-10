package afind

// Filters used to normalize paths for search responses

import (
	"regexp"
)

type pathFilter struct {
	Match *regexp.Regexp
	Replace []byte
}

func replaceWith(re, replace string) pathFilter {
	return pathFilter{regexp.MustCompile(re), []byte(replace)}
}

var (
	pathfilter = []pathFilter{
		// ...files named /buildroot/etc/ are really found in /etc/,
		// so reflect that:
		// replaceWith(`/buildroot/etc/`, `/etc/`),
		//
		// ...paths in /src but not files are found in /src
		// replaceWith(`/buildroot/src/(.*?)/`, `/src/$1/`),
	}
)
