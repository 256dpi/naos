package mqtt

import (
	"net/url"
	"strings"
)

func urlPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return strings.Trim(u.Path, "/")
}
