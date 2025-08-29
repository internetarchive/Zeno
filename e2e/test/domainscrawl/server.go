package domainscrawl

import (
	"net/http"
	"net/http/httptest"
)

func SetupServer() *httptest.Server {
	mux := http.NewServeMux()

	// Main page with links to other pages
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>Test Page</title></head>
<body>
<h1>Test Page</h1>
<p>This is the main test page.</p>
<a href="/level1">Level 1</a>
<a href="/external">External Link</a>
</body>
</html>`
		w.Write([]byte(html))
	})

	// Level 1 page (hop 1)
	mux.HandleFunc("/level1", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>Level 1</title></head>
<body>
<h1>Level 1</h1>
<p>This is level 1.</p>
<a href="/level2">Level 2</a>
<a href="/level1-2">Level 1-2</a>
</body>
</html>`
		w.Write([]byte(html))
	})

	// Level 2 page (hop 2) - should be skipped without domains-crawl
	mux.HandleFunc("/level2", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>Level 2</title></head>
<body>
<h1>Level 2</h1>
<p>This is level 2.</p>
<a href="/level3">Level 3</a>
</body>
</html>`
		w.Write([]byte(html))
	})

	// Another level 1 page
	mux.HandleFunc("/level1-2", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>Level 1-2</title></head>
<body>
<h1>Level 1-2</h1>
<p>This is another level 1 page.</p>
<a href="/level2-2">Level 2-2</a>
</body>
</html>`
		w.Write([]byte(html))
	})

	// Level 2-2 page (hop 2) - should be crawled with domains-crawl
	mux.HandleFunc("/level2-2", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>Level 2-2</title></head>
<body>
<h1>Level 2-2</h1>
<p>This is level 2-2.</p>
<a href="/level3-2">Level 3-2</a>
</body>
</html>`
		w.Write([]byte(html))
	})

	// Level 3-2 page (hop 3) - should be crawled with domains-crawl
	mux.HandleFunc("/level3-2", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>Level 3-2</title></head>
<body>
<h1>Level 3-2</h1>
<p>This is level 3-2.</p>
</body>
</html>`
		w.Write([]byte(html))
	})

	// External link (should not be crawled)
	mux.HandleFunc("/external", func(w http.ResponseWriter, r *http.Request) {
		html := `<html>
<head><title>External</title></head>
<body>
<h1>External</h1>
<p>This is an external link.</p>
</body>
</html>`
		w.Write([]byte(html))
	})

	return httptest.NewServer(mux)
}
