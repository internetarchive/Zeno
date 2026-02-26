package extractor

import (
	"fmt"

	"github.com/internetarchive/Zeno/pkg/models"

	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfmodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func init() {
	// https://pkg.go.dev/github.com/pdfcpu/pdfcpu@v0.9.1/pkg/pdfcpu/model#ConfigPath
	// > If you want to disable config dir usage in a multi threaded environment you are encouraged to use api.DisableConfigDir().
	pdfapi.DisableConfigDir()
}

type PDFOutlinkExtractor struct{}

func (PDFOutlinkExtractor) Support(m Mode) bool {
	return m == ModeGeneral
}

func (PDFOutlinkExtractor) Match(URL *models.URL) bool {
	return URL.GetMIMEType().Is("application/pdf")
}

func (PDFOutlinkExtractor) Extract(URL *models.URL) (outlinks []*models.URL, err error) {
	defer URL.RewindBody()
	defer func() {
		if r := recover(); r != nil {
			// TODO: remove this workaround once an new version of pdfcpu is released
			// https://github.com/pdfcpu/pdfcpu/issues/1193
			err = fmt.Errorf("pdf outlink extractor panicked: %v", r)
		}
	}()

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
			outlinks = append(outlinks, &models.URL{Raw: link.URI})
		}
	}

	return outlinks, nil
}
