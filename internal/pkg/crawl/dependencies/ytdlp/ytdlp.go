package ytdlp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

func GetJSON(port int) (URLs []string, err error) {
	// Prepare the command
	cmd := exec.Command("yt-dlp", "--dump-json", "http://localhost:"+strconv.Itoa(port))

	// Buffers to capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()
	if err != nil {
		return URLs, fmt.Errorf("yt-dlp error: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()

	// Find subtitles
	subtitleURLs, err := parseSubtitles(output)
	if err != nil {
		return nil, err
	}

	// Parse the output as a Video object
	var video Video
	err = json.Unmarshal([]byte(output), &video)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling yt-dlp JSON: %v", err)
	}

	// Get all thumbnail URLs
	for _, thumbnail := range video.Thumbnails {
		URLs = append(URLs, thumbnail.URL)
	}

	// Get the manifest URL for the best video & audio quality
	// Note: we do not archive live streams
	if !video.IsLive {
		for format := range video.RequestedFormats {
			URLs = append(URLs, video.RequestedFormats[format].URL)
		}
	}

	URLs = append(URLs, subtitleURLs...)

	return URLs, nil
}

func FindPath() (string, bool) {
	path, err := exec.LookPath("yt-dlp")
	if err != nil {
		return "", false
	}
	return path, true
}
