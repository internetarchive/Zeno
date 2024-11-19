package preprocessor

import "github.com/internetarchive/Zeno/pkg/models"

func validateURL(URL *models.URL, parentURL *models.URL) (err error) {
	// Validate the URL, REMOVE FRAGMENTS, try to fix it, make it absolute if needed, etc.
	return URL.Parse()
}
