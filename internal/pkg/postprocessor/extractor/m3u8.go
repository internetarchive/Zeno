package extractor

import (
	"github.com/grafov/m3u8"
	"github.com/internetarchive/Zeno/pkg/models"
)

func IsM3U8(URL *models.URL) bool {
	return URL.GetMIMEType() != nil &&
		URL.GetMIMEType().Is("application/vnd.apple.mpegurl") || URL.GetMIMEType().Is("application/x-mpegURL")
}

func M3U8(URL *models.URL) (assets []*models.URL, err error) {
	defer URL.RewindBody()

	var rawAssets ([]string)

	playlist, listType, err := m3u8.DecodeFrom(URL.GetBody(), true)
	if err != nil {
		return assets, err
	}

	switch listType {
	case m3u8.MEDIA:
		mediapl := playlist.(*m3u8.MediaPlaylist)

		for _, segment := range mediapl.Segments {
			if segment != nil && segment.URI != "" {
				rawAssets = append(rawAssets, segment.URI)
			}
		}
	case m3u8.MASTER:
		masterpl := playlist.(*m3u8.MasterPlaylist)

		for _, variant := range masterpl.Variants {
			if variant != nil {
				if variant.URI != "" {
					rawAssets = append(rawAssets, variant.URI)
				}

				for _, alt := range variant.Alternatives {
					if alt != nil && alt.URI != "" {
						rawAssets = append(rawAssets, alt.URI)
					}
				}
			}
		}
	}

	for _, rawAsset := range rawAssets {
		assets = append(assets, &models.URL{
			Raw: rawAsset,
		})
	}

	return assets, nil
}
