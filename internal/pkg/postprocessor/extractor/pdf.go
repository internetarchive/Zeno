package extractor

import (
	"errors"
	"log"
	"strings"

	"github.com/internetarchive/Zeno/pkg/models"

	"github.com/unidoc/unipdf/v3/core"
	pdf "github.com/unidoc/unipdf/v3/model"
)

func IsPDF(URL *models.URL) bool {
	return URL.GetMIMEType().Is("application/pdf")
}

func PDF(URL *models.URL) (outlinks []*models.URL, err error) {
	defer URL.RewindBody()

	pdfReader, err := pdf.NewPdfReader(URL.GetBody())
	if err != nil {
		return nil, err
	}

	// Parse PDF file.
	isEncrypted, err := pdfReader.IsEncrypted()
	if err != nil {
		return nil, err
	}

	// If PDF is encrypted, exit with message.
	if isEncrypted {
		return nil, errors.New("pdf is encrypted")
	}

	// Get number of pages.
	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		log.Fatal(err)
	}

	// Iterate through pages and print text.
	for i := 1; i <= numPages; i++ {
		page, err := pdfReader.GetPage(i)
		if err != nil {
			log.Fatal(err)
		}
		annotations, err := page.GetAnnotations()
		if err != nil {
			log.Fatal(err)
		}
		for _, annotation := range annotations {
			switch t := annotation.GetContext().(type) {
			case *pdf.PdfAnnotationLink:
				dict, ok := core.GetDict(t.A)
				if !ok {
					continue
				}

				url, ok := dict.GetString("URI")
				if !ok {
					continue
				}

				// Skip mailto and file links
				if strings.HasPrefix(url, "mailto:") || strings.HasPrefix(url, "file://") {
					continue
				}

				outlinks = append(outlinks, &models.URL{
					Raw: url,
				})
			}
		}
	}

	return outlinks, nil
}
