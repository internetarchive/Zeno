package crawl

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"
)

func (crawl *Crawl) initHTTPClient() (err error) {
	var customTransport = &http.Transport{
		Proxy:        nil,
		TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          30,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	var customClient = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: customTransport,
	}

	crawl.Client = customClient

	// Set proxy if one is specified
	if len(crawl.Proxy) > 0 {
		proxyURL, err := url.Parse(crawl.Proxy)
		if err != nil {
			return err
		}
		customTransport.Proxy = http.ProxyURL(proxyURL)
		customClient.Transport = customTransport
		crawl.ClientProxied = customClient
	}

	return nil
}
