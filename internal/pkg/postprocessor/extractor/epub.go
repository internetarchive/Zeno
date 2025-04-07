package extractor

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/pkg/models"
)

var epubStyleURLRegex = regexp.MustCompile(`url\(['"]?([^'"]+)['"]?\)`)

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

// EPUBOutlinks extracts outlinks from an EPUB file
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

		if !slices.Contains(config.Get().DisableHTMLTag, "a") {
			doc.Find("a").Each(func(index int, selection *goquery.Selection) {
				if href, exists := selection.Attr("href"); exists {
					if strings.HasPrefix(href, "http") {
						outlinks = append(outlinks, &models.URL{
							Raw: href,
						})
					}
				}
			})
		}
		htmlFile.Close()
	}
	return outlinks, nil
}

// EPUBAssets extracts assets from an EPUB file
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
	extractedAssets := make(map[string]bool)

	for _, manifest := range content.Manifests {
		if strings.Contains(manifest.MediaType, "html") || strings.Contains(manifest.MediaType, "xhtml") {
			continue
		}
		if strings.Contains(manifest.MediaType, "image") ||
			strings.Contains(manifest.MediaType, "audio") ||
			strings.Contains(manifest.MediaType, "video") ||
			strings.Contains(manifest.MediaType, "font") ||
			strings.Contains(manifest.MediaType, "css") {

			assetPath := ensureBasePath(basePath, manifest.Href)

			if !extractedAssets[assetPath] {
				assets = append(assets, &models.URL{
					Raw: assetPath,
				})
				extractedAssets[assetPath] = true
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

			if !slices.Contains(config.Get().DisableHTMLTag, "img") {
				doc.Find("img").Each(func(index int, selection *goquery.Selection) {
					if src, exists := selection.Attr("src"); exists {
						var assetURL string
						if strings.HasPrefix(src, "http") {
							assetURL = src
						} else {
							resolvedPath := resolveEPUBPath(htmlPath, src)
							assetURL = ensureBasePath(basePath, resolvedPath)
						}

						if !extractedAssets[assetURL] {
							assets = append(assets, &models.URL{
								Raw: assetURL,
							})
							extractedAssets[assetURL] = true
						}
					}
				})
			}

			var mediaSelectors []string
			if !slices.Contains(config.Get().DisableHTMLTag, "audio") {
				mediaSelectors = append(mediaSelectors, "audio[src]")
			}
			if !slices.Contains(config.Get().DisableHTMLTag, "video") {
				mediaSelectors = append(mediaSelectors, "video[src]")
			}
			if len(mediaSelectors) > 0 {
				doc.Find(strings.Join(mediaSelectors, ", ")).Each(func(index int, selection *goquery.Selection) {
					if src, exists := selection.Attr("src"); exists {
						var assetURL string
						if strings.HasPrefix(src, "http") {
							assetURL = src
						} else {
							resolvedPath := resolveEPUBPath(htmlPath, src)
							assetURL = ensureBasePath(basePath, resolvedPath)
						}
						if !extractedAssets[assetURL] {
							assets = append(assets, &models.URL{
								Raw: assetURL,
							})
							extractedAssets[assetURL] = true
						}
					}
				})
			}

			if !slices.Contains(config.Get().DisableHTMLTag, "link") {
				doc.Find("link[rel='stylesheet']").Each(func(index int, selection *goquery.Selection) {
					if href, exists := selection.Attr("href"); exists {
						var assetURL string
						if strings.HasPrefix(href, "http") {
							assetURL = href
						} else {
							resolvedPath := resolveEPUBPath(htmlPath, href)
							assetURL = ensureBasePath(basePath, resolvedPath)
						}
						if !extractedAssets[assetURL] {
							assets = append(assets, &models.URL{
								Raw: assetURL,
							})
							extractedAssets[assetURL] = true
						}

						if !strings.HasPrefix(href, "http") {
							cssFilePath := strings.TrimPrefix(assetURL, basePath+"/")
							cssFile, err := reader.Open(filepath.Join(basePath, cssFilePath))
							if err == nil {
								defer cssFile.Close()
								cssContent, err := io.ReadAll(cssFile)
								if err == nil {
									cssText := string(cssContent)
									cssURLs := epubStyleURLRegex.FindAllStringSubmatch(cssText, -1)
									for _, match := range cssURLs {
										if len(match) > 1 {
											url := match[1]
											var cssAssetURL string
											if strings.HasPrefix(url, "http") {
												cssAssetURL = url
											} else {
												cssBaseDir := filepath.Dir(cssFilePath)
												resolvedPath := resolveEPUBPath(filepath.Join(basePath, cssBaseDir), url)
												cssAssetURL = ensureBasePath(basePath, resolvedPath)
											}
											if !extractedAssets[cssAssetURL] {
												assets = append(assets, &models.URL{
													Raw: cssAssetURL,
												})
												extractedAssets[cssAssetURL] = true
											}
										}
									}
								}
							}
						}
					}
				})
			}
			htmlFile.Close()
		}
	}
	return assets, nil
}

// resolveEPUBPath resolves a relative path in an EPUB file
func resolveEPUBPath(basePath, relativePath string) string {
	if strings.HasPrefix(relativePath, "../") || strings.HasPrefix(relativePath, "./") {
		baseDir := filepath.Dir(basePath)
		return filepath.Clean(filepath.Join(baseDir, relativePath))
	}
	return relativePath
}

// ensureBasePath ensures that the asset path includes the basePath prefix
func ensureBasePath(basePath, assetPath string) string {
	cleanBasePath := filepath.Clean(basePath)
	cleanAssetPath := filepath.Clean(assetPath)

	testBasePath := cleanBasePath + string(filepath.Separator)
	if strings.HasPrefix(cleanAssetPath, testBasePath) {
		return cleanAssetPath
	}

	basePathName := filepath.Base(cleanBasePath)
	testBasePathName := basePathName + string(filepath.Separator)
	
	if strings.HasPrefix(cleanAssetPath, testBasePathName) {
		return filepath.Join(cleanBasePath, strings.TrimPrefix(cleanAssetPath, testBasePathName))
	}

	return filepath.Join(cleanBasePath, cleanAssetPath)
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