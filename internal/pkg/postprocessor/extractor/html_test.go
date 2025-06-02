package extractor

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/internetarchive/Zeno/internal/pkg/archiver"
	"github.com/internetarchive/Zeno/pkg/models"
)

func setupItem(html string) *models.Item {
	resp := &http.Response{
		Body: io.NopCloser(bytes.NewBufferString(html)),
	}
	newURL, err := models.NewURL("http://ex.com")
	if err != nil {
		panic(err)
	}
	newURL.SetResponse(resp)
	if err := archiver.ProcessBody(&newURL, false, false, 0, os.TempDir()); err != nil {
		panic(err)
	}
	return models.NewItem(&newURL, "")
}

func TestHTMLOutlinks(t *testing.T) {
	html := `
	<html>
		<head></head>
		<body>
			<a href="http://example.com">ex</a>
			<a href="http://archive.org">ar</a>
			<p>test</p>
			<a href="https://web.archive.org">wa</a>
			<a onclick="window.location='http://foo.com'">click me</a>
			<iframe title="Internet Archive" src="https://web.archive.org"></iframe>
		</body>
	</html>`
	item := setupItem(html)

	outlinks, err := HTMLOutlinks(item)
	if err != nil {
		t.Errorf("Error extracting HTML outlinks %s", err)
	}
	if len(outlinks) != 5 {
		t.Errorf("We couldn't extract all HTML outlinks. Received %d, expected 5", len(outlinks))
	}
}

// Test <audio> and <video> src extraction
func TestHTMLAssetsAudioVideo(t *testing.T) {
	html := `
	<html>
		<body>
			<video src="http://f1.com"></video>
			<p>test</p>
			<audio src="http://f2.com"></audio>
		</body>
	</html>`
	item := setupItem(html)

	assets, err := HTMLAssets(item)
	if err != nil {
		t.Errorf("HTMLAssets error = %v", err)
	}
	if len(assets) != 2 {
		t.Errorf("We couldn't extract all audio/video assets. Received %d, expected 2", len(assets))
	}
}

// Test [data-item], [style], [data-preview] attribute extraction
func TestHTMLAssetsAttributes(t *testing.T) {
	html := `
	<html>
		<body>
		 <div style="background: url('http://something.com/data.jpg')"></div>
	   <div data-preview="http://archive.org">...</div>
			<p>test</p>
			<div data-item='{"id": 123, "name": "Sample Item", "image": "https://example.com/image.jpg"}'>
    		Click here for details
			</div>
		</body>
	</html>`
	item := setupItem(html)

	assets, err := HTMLAssets(item)
	if err != nil {
		t.Errorf("HTMLAssets error = %v", err)
	}
	if len(assets) != 3 {
		t.Errorf("We couldn't extract all [data-item], [style], [data-preview] attribute assets. Received %d, expected 3", len(assets))
	}
}

// Must get link.href, ignore rel="alternate" href, get meta.href and get 3 URLs from meta refresh.
func TestHTMLAssetsMeta(t *testing.T) {
	html := `
	<html>
		<head>
			<link rel="stylesheet" href="http://ex.com/styles/styles.7f7c9ce840c7e527.css">
			<!-- ignore because of rel="alternate" -->
			<link rel="alternate" href="http://ex.com/styles/styles.7f7c9ce840c7e527.css">
			<link foo="123" bar="456">
			<meta href="https://a1.com">
			<meta content="something">
			<meta http-equiv="Refresh" content="0; url=https://refr1.com">
			<meta http-equiv="REFRESH" content="0; url=HTTP://UPPER.COM/PAGE2.HTML">
			<meta http-equiv="Refresh" content="https://refr2.com">
			<meta http-equiv="refresh" content="5">
			<meta http-equiv="Refresh" content="0; url='https://quote.tld/1'">
			<meta http-equiv='Refresh' content='0; url=  "https://quote.tld/2" '>
		</head>
		<body>
			experiment
		</body>
	</html>`
	urlsExpected := []string{
		"http://ex.com/styles/styles.7f7c9ce840c7e527.css",
		"https://a1.com",
		"https://refr1.com",
		"http://UPPER.COM/PAGE2.HTML",
		"https://refr2.com",
		"https://quote.tld/1",
		"https://quote.tld/2",
	}
	item := setupItem(html)

	assets, err := HTMLAssets(item)
	if err != nil {
		t.Errorf("HTMLAssets error = %v", err)
	}
	if len(assets) != 7 {
		t.Errorf("We couldn't extract all meta & link assets. Received %d, expected 5", len(assets))
	}
	for i, asset := range assets {
		if asset.Raw != urlsExpected[i] {
			t.Errorf("Invalid URL extracted %v, expected %v", asset.Raw, urlsExpected[i])
		}
	}
}

func TestSrcset(t *testing.T) {
	html := `
	<html>
		<body>
		<img srcset="http://ex.com/a.jpg 480w, http://ex.com/b.jpg 800w"
		    sizes="(max-width: 600px) 480px, 800px"
			src="http://ex.com/c.jpg" />
		<picture>
		<source media="(min-width: 0px) and (-webkit-min-device-pixel-ratio: 1.25), (min-resolution: 120dpi)" sizes="95vw" srcset="https://example.com/5.jpg?w=460 460w, http://example.com/img/media/6/5.jpg 340w"/>
		</picture>
		</body>
	</html>`
	item := setupItem(html)
	assets, err := HTMLAssets(item)
	if err != nil {
		t.Errorf("Error extracting HTML assets %s", err)
	}
	if len(assets) != 5 {
		t.Errorf("We couldn't extract all assets. Extracted %d instead of 3.", len(assets))
	}
	if assets[0].Raw != "http://ex.com/c.jpg" {
		t.Errorf("Invalid img URL extracted %v", assets[0].Raw)
	}
	if assets[1].Raw != "http://ex.com/a.jpg" {
		t.Errorf("Invalid img URL extracted %v", assets[1].Raw)
	}
	if assets[2].Raw != "http://ex.com/b.jpg" {
		t.Errorf("Invalid img URL extracted %v", assets[2].Raw)
	}
}

func TestUpperCase(t *testing.T) {
	html := `
	<HTML>
	   <BODY>
	   <A HREF="https://a.com/a.html">text</A>
	   </BODY>
    </HTML>`
	item := setupItem(html)
	outlinks, err := HTMLOutlinks(item)
	if err != nil {
		t.Errorf("Error extracting HTML outlinks %s", err)
	}
	if len(outlinks) != 1 {
		t.Errorf("We couldn't extract all HTML outlinks. Extracted %d instead of 1", len(outlinks))
	}
}

func TestCSS(t *testing.T) {
	html := `<html>
	<head>
	  <style type="text/css">
	  #head{
	    background:transparent url(http://g.org/images/logo.jpg);
   	  }
	  #footer{
		background-image:url(http://m.gr/genbg?a=2&amp;b=1);
	  }
	  @import 'http://foo.org/common.css';
	  </style>
    </head>
	<body>
	  <div style="background: url(http://n.ua/img/bg.png);">
	</body>
    </html>`
	item := setupItem(html)
	assets, err := HTMLAssets(item)
	if err != nil {
		t.Errorf("Error extracting HTML assets %s", err)
	}
	if len(assets) != 3 {
		t.Errorf("We couldn't extract all HTML assets. Extracted %d instead of 3", len(assets))
	}
}
