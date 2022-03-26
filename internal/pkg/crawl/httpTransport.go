package crawl

// type customTransport struct {
// 	http.Transport
// 	c   *Crawl
// 	d   *customDialer
// 	h2t *http2.Transport
// }

func isRedirection(statusCode int) bool {
	if statusCode == 300 || statusCode == 301 ||
		statusCode == 302 || statusCode == 307 ||
		statusCode == 308 {
		return true
	}
	return false
}

// func (t *customTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
// 	// Use httptrace to increment the URI/s counter on DNS requests.
// 	trace := &httptrace.ClientTrace{
// 		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
// 			t.c.URIsPerSecond.Incr(1)
// 		},
// 	}
// 	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

// 	req.Header.Set("User-Agent", t.c.UserAgent)
// 	req.Header.Set("Accept-Encoding", "gzip")

// 	// Retry on request errors and rate limiting.
// 	var sleepTime = time.Millisecond * 250
// 	var exponentFactor = 2
// 	for i := 0; i <= t.c.MaxRetry; i++ {
// 		t.c.URIsPerSecond.Incr(1)

// 		if i != 0 {
// 			resp.Body.Close()
// 		}

// 		// conn, err := t.d.DialRequest(req)
// 		// if err != nil {
// 		// 	return nil, err
// 		// }

// 		// h2c, err := t.h2t.NewClientConn(conn)
// 		// if err != nil {
// 		// 	panic(err)
// 		// 	return nil, err
// 		// }

// 		resp, err = t.Transport.RoundTrip(req)
// 		if err != nil {
// 			logWarning.WithFields(logrus.Fields{
// 				"url":   req.URL.String(),
// 				"error": err,
// 			}).Warning("HTTP error")
// 			return resp, err
// 		}

// 		// If the crawl is finishing, we do not want to sleep and retry anymore.
// 		if t.c.Finished.Get() {
// 			return resp, err
// 		}

// 		// Check for status code. When we encounter an error or some rate limiting,
// 		// we exponentially backoff between retries.
// 		if string(strconv.Itoa(resp.StatusCode)[0]) != "2" && isRedirection(resp.StatusCode) == false {
// 			// If we get a 404, we do not waste any time retrying
// 			if resp.StatusCode == 404 {
// 				return resp, nil
// 			}

// 			// If we get a 429, then we are being rate limited, in this case we
// 			// sleep then retry.
// 			// TODO: If the response include the "Retry-After" header, we use it to sleep for the appropriate time before retrying.
// 			if resp.StatusCode == 429 {
// 				sleepTime = sleepTime * time.Duration(exponentFactor)
// 				logInfo.WithFields(logrus.Fields{
// 					"url":         req.URL.String(),
// 					"duration":    sleepTime.String(),
// 					"retry_count": i,
// 					"status_code": resp.StatusCode,
// 				}).Info("We are being rate limited, sleeping then retrying..")
// 				time.Sleep(sleepTime)
// 				continue
// 			}

// 			// If we get any other error, we simply wait for a random time between
// 			// 0 and 1s, then retry.
// 			rand.Seed(time.Now().UnixNano())
// 			time.Sleep(time.Millisecond * time.Duration(rand.Intn(1000)))
// 			continue
// 		}
// 		return resp, nil
// 	}
// 	return resp, nil
// }
