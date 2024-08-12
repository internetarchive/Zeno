package ytdlp

import (
	"encoding/json"
	"fmt"
)

type SubtitleInfo struct {
	Ext  string `json:"ext"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

// parseSubtitles parses the subtitles from the yt-dlp JSON output,
// it's needed because the subtitles are not given as a proper array or objects
func parseSubtitles(jsonData string) ([]string, error) {
	var data map[string]json.RawMessage
	err := json.Unmarshal([]byte(jsonData), &data)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling outer JSON: %v", err)
	}

	subtitlesRaw, ok := data["subtitles"]
	if !ok {
		return nil, nil
	}

	var subtitles map[string][]SubtitleInfo
	err = json.Unmarshal(subtitlesRaw, &subtitles)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling subtitles JSON: %v", err)
	}

	var URLs []string
	for _, langSubtitles := range subtitles {
		for _, subtitle := range langSubtitles {
			URLs = append(URLs, subtitle.URL)
		}
	}

	return URLs, nil
}
