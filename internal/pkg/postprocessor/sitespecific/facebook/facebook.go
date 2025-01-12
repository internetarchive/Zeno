package facebook

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/internetarchive/Zeno/pkg/models"
)

func IsFacebookPostURL(URL *models.URL) bool {
	return strings.Contains(URL.String(), "www.facebook.com") &&
		strings.Contains(URL.String(), "/posts/") &&
		!strings.Contains(URL.String(), "/plugins/post.php")
}

func GenerateEmbedURL(URL *models.URL) *models.URL {
	return &models.URL{
		Raw:  fmt.Sprintf("https://www.facebook.com/plugins/post.php?href=%s&show_text=true", url.QueryEscape(URL.String())),
		Hops: URL.GetHops(),
	}
}
