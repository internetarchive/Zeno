package crawl

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strconv"
	"time"

	"github.com/CorentinB/warc"
	"github.com/sirupsen/logrus"
)

type customTransport struct {
	http.Transport
	c *Crawl
}

func isRedirection(statusCode int) bool {
	if statusCode == 300 || statusCode == 301 ||
		statusCode == 302 || statusCode == 307 ||
		statusCode == 308 {
		return true
	}
	return false
}

func (t *customTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	var records *warc.RecordBatch

	// Use httptrace to increment the URI/s counter on DNS requests
	trace := &httptrace.ClientTrace{
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			t.c.URIsPerSecond.Incr(1)
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	req.Header.Set("User-Agent", t.c.UserAgent)
	req.Header.Set("Accept-Encoding", "*/*")

	// Retry on request error, 5xx  and rate limiting
	var sleepTime = time.Millisecond * 250
	var exponentFactor = 2
	for i := 0; i <= t.c.MaxRetry; i++ {
		t.c.URIsPerSecond.Incr(1)
		resp, err = http.DefaultTransport.RoundTrip(req)
		if err != nil {
			logWarning.WithFields(logrus.Fields{
				"url":   req.URL.String(),
				"error": err,
			}).Warning("HTTP error")
			continue
		}

		// Write response and request
		if t.c.WARC {
			records, err = warc.RecordsFromHTTPResponse(resp)
			if err != nil {
				logWarning.WithFields(logrus.Fields{
					"url":   req.URL.String(),
					"error": err,
				}).Warning("Error turning http.Resp into WARC records")
				resp.Body.Close()
				continue
			}
			t.c.WARCWriter <- records
			t.c.Crawled.Incr(1)
		}

		// If the crawl is finishing, we do not want to sleep and retry anymore
		if t.c.Finished.Get() {
			break
		}

		// Check for status code. When we encounter an error or some rate limiting,
		// we exponentially backoff between retries.
		if string(strconv.Itoa(resp.StatusCode)[0]) != "2" && isRedirection(resp.StatusCode) == false {
			resp.Body.Close()
			sleepTime = sleepTime * time.Duration(exponentFactor)
			logInfo.WithFields(logrus.Fields{
				"url":         req.URL.String(),
				"duration":    sleepTime.String(),
				"retry_count": i,
				"status_code": resp.StatusCode,
			}).Info("Error or rate limiting, sleeping then retrying..")
			time.Sleep(sleepTime)
			continue
		}
		break
	}
	return resp, err
}

func (crawl *Crawl) initHTTPClient() (err error) {
	var customTransport = new(customTransport)

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
		DualStack: true,
	}).DialContext

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
