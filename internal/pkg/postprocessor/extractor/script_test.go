package extractor

import (
	"testing"
)

func TestExtractFromScriptContent(t *testing.T) {
	// Sample script content with a fake URL
	scriptContent := `
	/* <![CDATA[ */
	var welcomebar_frontjs = {"ajaxurl":"http:\/\/fakeurl.invalid\/wp-admin\/admin-ajax.php","days":"Days","hours":"Hours","minutes":"Minutes","seconds":"Seconds","ajax_nonce":"c35d389da5"};
	/* ]]> */
	`

	expected := "http://fakeurl.invalid/wp-admin/admin-ajax.php"
	assets, err := extractFromScriptContent(scriptContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}

	if assets[0] != expected {
		t.Errorf("expected asset %q, got %q", expected, assets[0])
	}
}
