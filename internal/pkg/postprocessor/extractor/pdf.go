package extractor

import (
	"strings"

	"github.com/internetarchive/Zeno/pkg/models"

	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfmodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func init() {
	// https://pkg.go.dev/github.com/pdfcpu/pdfcpu@v0.9.1/pkg/pdfcpu/model#ConfigPath
	// > If you want to disable config dir usage in a multi threaded environment you are encouraged to use api.DisableConfigDir().
	pdfapi.DisableConfigDir()
}

func IsPDF(URL *models.URL) bool {
	return URL.GetMIMEType().Is("application/pdf")
}

func PDF(URL *models.URL) (outlinks []*models.URL, err error) {
	defer URL.RewindBody()

	annots, err := pdfapi.Annotations(URL.GetBody(), nil, nil)
	if err != nil {
		return nil, err
	}

	for _, pageAnnots := range annots {
		linkAnnots, ok := pageAnnots[pdfmodel.AnnLink]
		if !ok {
			continue
		}

		for _, renderer := range linkAnnots.Map {
			link, ok := renderer.(pdfmodel.LinkAnnotation)
			if !ok || link.URI == "" {
				continue
			}

			// Skip unwanted URIs
			if strings.HasPrefix(link.URI, "mailto:") ||
				strings.HasPrefix(link.URI, "tel:") ||
				strings.HasPrefix(link.URI, "file:") {
				continue
			}

			outlinks = append(outlinks, &models.URL{Raw: link.URI})
		}
	}

	return outlinks, nil
}
