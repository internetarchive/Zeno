package models

import (
	"net/url"

	"github.com/internetarchive/gocrawlhq"
)

type URL struct {
	gocrawlhq.URL
}

func (u *URL) Parsed() (URL *url.URL, err error) {
	return url.Parse(u.Value)
}
