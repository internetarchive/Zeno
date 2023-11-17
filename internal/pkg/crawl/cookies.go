package crawl

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"

	"github.com/mengzhuo/cookiestxt"
)

// loadCookiesFromFile loads cookies from a cookies.txt file into the crawl's cookie jar.
// TODO: proxied client is not handled here
func (c *Crawl) loadCookiesFromFile() (err error) {
	// Create a cookie jar
	c.Client.Jar, err = cookiejar.New(&cookiejar.Options{
		PublicSuffixList: nil,
	})
	if err != nil {
		return err
	}

	// Open the cookies file
	f, err := os.OpenFile(c.CookieFile, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}

	// Read the cookies file
	buf, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	// Parse the cookies file
	cookies, err := cookiestxt.Parse(bytes.NewReader(buf))
	if err != nil {
		return err
	}

	// Add the cookies to the jar
	c.addCookiesToJar(cookies)

	// // URL for which you want to retrieve cookies
	// targetURL, _ := url.Parse("https://www.facebook.com/Hannah.GilesKnopp")

	// // Retrieve cookies for the specified URL
	// fbcookies := c.CookieJar.Cookies(targetURL)
	// for _, cookie := range fbcookies {
	// 	fmt.Printf("Cookie: %s = %s\n", cookie.Name, cookie.Value)
	// }

	return nil
}

// addCookiesToJar adds cookies to the specified cookie jar, grouped by each cookie's domain.
func (c *Crawl) addCookiesToJar(cookies []*http.Cookie) {
	// Group cookies by domain
	domainCookies := make(map[string][]*http.Cookie)
	for _, cookie := range cookies {
		if cookie.Domain != "" {
			domainCookies[cookie.Domain] = append(domainCookies[cookie.Domain], cookie)
		}
	}

	// Add the grouped cookies to the jar
	for domain, cookies := range domainCookies {
		urlStr := fmt.Sprintf("http://%s", domain)
		parsedURL, err := url.Parse(urlStr)
		if err != nil {
			fmt.Printf("Error parsing URL from domain: %s\n", err)
			continue
		}

		if c.Client.Jar != nil {
			c.Client.Jar.SetCookies(parsedURL, cookies)
		}
	}
}
