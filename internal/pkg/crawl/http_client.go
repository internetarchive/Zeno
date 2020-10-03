package crawl

import (
	"crypto/tls"
	"net/http"
	"path"
	"time"

	"github.com/CorentinB/warc"
	"github.com/go-resty/resty/v2"
	"github.com/gojektech/heimdall/v6"
	"github.com/gojektech/heimdall/v6/httpclient"
	log "github.com/sirupsen/logrus"
)

type customHTTPClient struct {
	client           *resty.Client
	warcWriteChannel chan *warc.RecordBatch
}

func (c *customHTTPClient) Do(request *http.Request) (resp *http.Response, err error) {
	resp, err = c.client.GetClient().Do(request)
	if err != nil {
		return resp, err
	}

	// Write response and request
	records, err := warc.RecordsFromHTTPResponse(resp)
	if err != nil {
		log.WithFields(log.Fields{
			"url":   request.URL.String(),
			"error": err,
		}).Error("error when turning HTTP resp into WARC records")
		return resp, err
	}
	c.warcWriteChannel <- records

	return resp, nil
}

// InitHTTPClient intialize HTTP client
func (crawl *Crawl) InitHTTPClient() (err error) {
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
		customClient := new(customHTTPClient)
		customClient.client = resty.New()

		// Set Socks5 proxy if one is specified
		if len(crawl.Proxy) > 0 {
			customClient.client.SetProxy("socks5://" + crawl.Proxy)
		}

		// Initialize WARC writer if --warc is specified
		if crawl.WARC {
			var rotatorSettings = warc.NewRotatorSettings()
			rotatorSettings.OutputDirectory = path.Join(crawl.JobPath, "warcs")
			rotatorSettings.Compression = "GZIP"
			rotatorSettings.Prefix = "ZENO"

			crawl.WARCWriter, crawl.WARCWriterFinish, err = rotatorSettings.NewWARCRotator()
			if err != nil {
				return err
			}

			// Disable HTTP/2: Empty TLSNextProto map
			customClient.client.GetClient().Transport = http.DefaultTransport
			customClient.client.GetClient().Transport.(*http.Transport).TLSNextProto =
				make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)

			customClient.warcWriteChannel = crawl.WARCWriter
		}

		clientOptions = append(clientOptions, httpclient.WithHTTPClient(customClient))
	}

	crawl.Client = httpclient.NewClient(clientOptions...)

	return nil
}
