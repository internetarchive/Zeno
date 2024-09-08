package ytdlp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func GetJSON(port int) (URLs []string, rawJSON string, err error) {
	// Prepare the command
	cmd := exec.Command("yt-dlp", "--dump-json", "-f", "18", "http://localhost:"+strconv.Itoa(port))

	// Buffers to capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()
	if err != nil {
		return URLs, rawJSON, fmt.Errorf("yt-dlp error: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()

	// Find subtitles
	subtitleURLs, err := parseSubtitles(output)
	if err != nil {
		return nil, rawJSON, fmt.Errorf("error parsing subtitles: %v", err)
	}

	// Parse the output as a Video object
	var video Video
	err = json.Unmarshal([]byte(output), &video)
	if err != nil {
		return nil, rawJSON, fmt.Errorf("error unmarshaling yt-dlp JSON: %v", err)
	}

	// Get all thumbnail URLs
	for _, thumbnail := range video.Thumbnails {
		URLs = append(URLs, thumbnail.URL)
	}

	// Get the manifest URL for the best video & audio quality
	// Note: we do not archive live streams
	if !video.IsLive {
		// Find the best format for the video in the formats that
		// use the "https" protocol and don't contain "only" in their name (to avoid audio or video-only formats)
		// and don't contain "_dash" in their container (to avoid DASH formats)
		var bestFormatQuality float64
		var bestFormatPosition int
		for i, format := range video.Formats {
			if (bestFormatQuality == 0 || format.Quality > bestFormatQuality) &&
				format.Protocol == "https" &&
				!strings.Contains(format.Format, "only") &&
				!strings.Contains(format.Container, "_dash") {
				bestFormatQuality = format.Quality
				bestFormatPosition = i
			}
		}

		URLs = append(URLs,
			video.Formats[bestFormatPosition].URL+"&video_id="+video.ID,
			video.Formats[bestFormatPosition].URL)
	}

	URLs = append(URLs, subtitleURLs...)

	return URLs, output, nil
}

func FindPath() (string, bool) {
	path, err := exec.LookPath("yt-dlp")
	if err != nil {
		return "", false
	}
	return path, true
}
