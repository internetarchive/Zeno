package crawl

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"github.com/gojektech/heimdall/v6"
	"github.com/gojektech/heimdall/v6/httpclient"
	"github.com/sirupsen/logrus"
)

// InitHTTPClient intialize HTTP client
func (crawl *Crawl) InitHTTPClient() (err error) {
	var customClient = new(http.Client)
	var customTransport = new(http.Transport)

	var maximumJitterInterval time.Duration = 2 * time.Millisecond // Max jitter interval
	var initalTimeout time.Duration = 2 * time.Millisecond         // Inital timeout
	var maxTimeout time.Duration = 9 * time.Millisecond            // Max time out
	var timeout time.Duration = 1000 * time.Millisecond
	var exponentFactor float64 = 2 // Multiplier

	backoff := heimdall.NewExponentialBackoff(initalTimeout, maxTimeout, exponentFactor, maximumJitterInterval)
	retrier := heimdall.NewRetrier(backoff)

	// Create a new client, sets the retry mechanism, and the number of retries
	var clientOptions []httpclient.Option
	clientOptions = append(clientOptions, httpclient.WithHTTPTimeout(timeout), httpclient.WithRetrier(retrier), httpclient.WithRetryCount(4))

	if crawl.WARC || len(crawl.Proxy) > 0 {
		// Initialize WARC writer if --warc is specified
		if crawl.WARC {
			logrus.Info("Initializing WARC writer pool..")
			crawl.initWARCWriterPool()
			logrus.Info("WARC writer pool initialized")

			// Disable HTTP/2: Empty TLSNextProto map
			customTransport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
		}

		// Set Socks5 proxy if one is specified
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
	clientOptions = append(clientOptions, httpclient.WithHTTPClient(customClient))
	crawl.Client = httpclient.NewClient(clientOptions...)

	return nil
}