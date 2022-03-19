package crawl

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/http2"
)

func (crawl *Crawl) initHTTPClient() (err error) {
	var customHTTPClient = new(http.Client)
	var customTransport = new(customTransport)
	var customDialer = new(customDialer)

	customDialer.c = crawl
	customDialer.Timeout = 30 * time.Second
	customDialer.KeepAlive = -1

	customTransport.c = crawl
	customTransport.Proxy = nil
	customTransport.MaxIdleConns = -1
	customTransport.IdleConnTimeout = -1
	customTransport.TLSHandshakeTimeout = 15 * time.Second
	customTransport.ExpectContinueTimeout = 1 * time.Second
	customTransport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
	customTransport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	customTransport.d = customDialer
	customTransport.Dial = customDialer.CustomDial_
	customTransport.DialTLS = customDialer.CustomDialTLS_

	customTransport.DisableCompression = true
	customTransport.ForceAttemptHTTP2 = false

	customHTTPClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	customHTTPClient.Transport = customTransport

	h2t, err := http2.ConfigureTransports(&customTransport.Transport)
	if err != nil {
		return err
	}
	customTransport.h2t = h2t

	crawl.Client = customHTTPClient

	// Set proxy if one is specified
	if len(crawl.Proxy) > 0 {
		proxyURL, err := url.Parse(crawl.Proxy)
		if err != nil {
			return err
		}
		customTransport.Proxy = http.ProxyURL(proxyURL)
		customHTTPClient.Transport = customTransport
		crawl.ClientProxied = customHTTPClient
	}

	return nil
}
