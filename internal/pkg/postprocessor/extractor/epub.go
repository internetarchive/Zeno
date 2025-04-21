package extractor

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
)

type EPUBContainerXML struct {
	XMLName  xml.Name `xml:"container"`
	RootFile struct {
		FullPath  string `xml:"full-path,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"rootfiles>rootfile"`
}

type EPUBContent struct {
	XMLName   xml.Name `xml:"package"`
	Manifests []struct {
		ID        string `xml:"id,attr"`
		Href      string `xml:"href,attr"`
		MediaType string `xml:"media-type,attr"`
	} `xml:"manifest>item"`
	Spine struct {
		ItemRefs []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
	Guide struct {
		References []struct {
			Type  string `xml:"type,attr"`
			Title string `xml:"title,attr"`
			Href  string `xml:"href,attr"`
		} `xml:"reference"`
	} `xml:"guide"`
}

func IsEPUB(URL *models.URL) bool {
	return isContentType(URL.GetResponse().Header.Get("Content-Type"), "application/epub+zip") ||
		strings.Contains(URL.GetMIMEType().String(), "epub")
}

func EPUBOutlinks(item *models.Item) (outlinks []*models.URL, err error) {
	defer item.GetURL().RewindBody()
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.extractor.EPUBOutlinks",
	})

	body := item.GetURL().GetBody()
	size, err := getFileSize(body)
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(body, size)
	if err != nil {
		return nil, err
	}

	containerFile, err := reader.Open("META-INF/container.xml")
	if err != nil {
		return nil, err
	}
	defer containerFile.Close()
	var container EPUBContainerXML
	if err := xml.NewDecoder(containerFile).Decode(&container); err != nil {
		return nil, err
	}
	contentPath := container.RootFile.FullPath
	contentFile, err := reader.Open(contentPath)
	if err != nil {
		return nil, err
	}
	defer contentFile.Close()
	var content EPUBContent
	if err := xml.NewDecoder(contentFile).Decode(&content); err != nil {
		return nil, err
	}

	basePath := filepath.Dir(contentPath)
	htmlFiles := make(map[string]string)

	for _, manifest := range content.Manifests {
		if strings.Contains(manifest.MediaType, "html") || strings.Contains(manifest.MediaType, "xhtml") {
			htmlPath := filepath.Join(basePath, manifest.Href)
			htmlFiles[manifest.ID] = htmlPath
		}
	}

	extractedOutlinksMap := make(map[string]bool)

	for _, htmlPath := range htmlFiles {
		htmlFile, err := reader.Open(htmlPath)
		if err != nil {
			logger.Debug("unable to open HTML file in EPUB", "error", err, "path", htmlPath, "item", item.GetShortID())
			continue
		}

		doc, err := goquery.NewDocumentFromReader(htmlFile)
		if err != nil {
			htmlFile.Close()
			logger.Debug("unable to parse HTML file in EPUB", "error", err, "path", htmlPath, "item", item.GetShortID())
			continue
		}

		rawLinks := ExtractOutlinksFromDocument(doc, "", nil)

		htmlFile.Close()

		for _, rawLink := range rawLinks {
			if strings.HasPrefix(rawLink, "http://") || strings.HasPrefix(rawLink, "https://") {
				if !extractedOutlinksMap[rawLink] {
					outlinks = append(outlinks, &models.URL{
						Raw: rawLink,
					})
					extractedOutlinksMap[rawLink] = true
				}
			}
		}
	}
	return outlinks, nil
}

func EPUBAssets(item *models.Item) (assets []*models.URL, err error) {
	defer item.GetURL().RewindBody()
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.extractor.EPUBAssets",
	})

	body := item.GetURL().GetBody()
	size, err := getFileSize(body)
	if err != nil {
		return nil, err
	}

	reader, err := zip.NewReader(body, size)
	if err != nil {
		return nil, err
	}

	containerFile, err := reader.Open("META-INF/container.xml")
	if err != nil {
		return nil, err
	}
	defer containerFile.Close()
	var container EPUBContainerXML
	if err := xml.NewDecoder(containerFile).Decode(&container); err != nil {
		return nil, err
	}

	contentPath := container.RootFile.FullPath
	contentFile, err := reader.Open(contentPath)
	if err != nil {
		return nil, err
	}
	defer contentFile.Close()
	var content EPUBContent
	if err := xml.NewDecoder(contentFile).Decode(&content); err != nil {
		return nil, err
	}

	basePath := filepath.Dir(contentPath)
	extractedAssetsMap := make(map[string]bool)

	for _, manifest := range content.Manifests {
		if strings.Contains(manifest.MediaType, "html") || strings.Contains(manifest.MediaType, "xhtml") {
			continue
		}
		if strings.Contains(manifest.MediaType, "image") ||
			strings.Contains(manifest.MediaType, "audio") ||
			strings.Contains(manifest.MediaType, "video") ||
			strings.Contains(manifest.MediaType, "font") ||
			strings.Contains(manifest.MediaType, "css") {

			assetPath := filepath.Join(basePath, manifest.Href)

			if !extractedAssetsMap[assetPath] {
				assets = append(assets, &models.URL{
					Raw: assetPath,
				})
				extractedAssetsMap[assetPath] = true
			}
		}
	}

	for _, manifest := range content.Manifests {
		if strings.Contains(manifest.MediaType, "html") || strings.Contains(manifest.MediaType, "xhtml") {
			htmlPath := filepath.Join(basePath, manifest.Href)
			htmlFile, err := reader.Open(htmlPath)
			if err != nil {
				logger.Debug("unable to open HTML file in EPUB", "error", err, "path", htmlPath, "item", item.GetShortID())
				continue
			}
			doc, err := goquery.NewDocumentFromReader(htmlFile)
			if err != nil {
				htmlFile.Close()
				logger.Debug("unable to parse HTML file in EPUB", "error", err, "path", htmlPath, "item", item.GetShortID())
				continue
			}

			currentHTMLBaseDirInEPUB := filepath.Dir(htmlPath)

			rawAssetLinks := ExtractAssetsFromDocument(doc, "", nil)

			htmlFile.Close()

			for _, rawLink := range rawAssetLinks {
				var assetURL string
				if strings.HasPrefix(rawLink, "http://") || strings.HasPrefix(rawLink, "https://") {
					assetURL = rawLink
				} else {
					assetURL = ensureBasePath(resolveEPUBPath(currentHTMLBaseDirInEPUB, rawLink))
				}

				if !extractedAssetsMap[assetURL] {
					assets = append(assets, &models.URL{
						Raw: assetURL,
					})
					extractedAssetsMap[assetURL] = true
				}
			}
		}
	}

	return assets, nil
}

func resolveEPUBPath(htmlBaseDir, relativePath string) string {
	return filepath.Clean(filepath.Join(htmlBaseDir, relativePath))
}

func ensureBasePath(assetPath string) string {
	cleanAssetPath := filepath.Clean(assetPath)

	if strings.HasPrefix(cleanAssetPath, "http://") || strings.HasPrefix(cleanAssetPath, "https://") {
		return cleanAssetPath
	}

	return filepath.Clean(assetPath)
}

// helper function using file.SEEK() to determine file size while ensuring the
// file pointer is reset to its original position
func getFileSize(file io.ReadSeeker) (int64, error) {
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	_, err = file.Seek(currentPos, io.SeekStart)
	if err != nil {
		return 0, err
	}

	return size, nil
}