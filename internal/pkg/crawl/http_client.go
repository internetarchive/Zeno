package crawl

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"time"
)

func (crawl *Crawl) initHTTPClient() (err error) {
	var customHTTPClient = new(http.Client)
	var customTransport = new(customTransport)
	var customDialer = new(customDialer)

	customDialer.c = crawl
	customDialer.Timeout = 30 * time.Second
	customDialer.KeepAlive = 30 * time.Second

	customTransport.c = crawl
	customTransport.Proxy = nil
	customTransport.MaxIdleConns = 30
	customTransport.IdleConnTimeout = 90 * time.Second
	customTransport.TLSHandshakeTimeout = 15 * time.Second
	customTransport.ExpectContinueTimeout = 1 * time.Second
	customTransport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
	customTransport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	customTransport.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext

	customTransport.DisableCompression = true
	customTransport.Dial = customDialer.CustomDial
	customTransport.DialTLS = customDialer.CustomDialTLS

	customHTTPClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	customHTTPClient.Transport = customTransport

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
