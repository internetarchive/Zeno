package crawl

import (
	"crypto/tls"
	"net/http"
	"net/url"

	"github.com/sirupsen/logrus"
)

func (crawl *Crawl) initHTTPClient() (err error) {
	var customTransport = new(http.Transport)
	var customClient = new(http.Client)

	if crawl.WARC || len(crawl.Proxy) > 0 {
		// Initialize WARC writer if --warc is specified
		if crawl.WARC {
			logrus.Info("Initializing WARC writer pool..")
			crawl.initWARCWriterPool()
			logrus.Info("WARC writer pool initialized")

			// Disable HTTP/2: Empty TLSNextProto map
			customTransport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
			customClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}
		}

		// Set proxy if one is specified
		if len(crawl.Proxy) > 0 {
			proxyURL, err := url.Parse(crawl.Proxy)
			if err != nil {
				return err
			}
			customTransport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	customClient.Transport = customTransport
	crawl.Client = customClient

	return nil
}
