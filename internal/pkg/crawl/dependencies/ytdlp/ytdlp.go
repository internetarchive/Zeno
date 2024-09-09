package ytdlp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

func GetJSON(port int) (URLs []string, rawJSON string, err error) {
	// Prepare the command
	cmd := exec.Command("yt-dlp", "--dump-json", "http://localhost:"+strconv.Itoa(port))

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
	// subtitleURLs, err := parseSubtitles(output)
	// if err != nil {
	// 	return nil, rawJSON, fmt.Errorf("error parsing subtitles: %v", err)
	// }

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
		for _, format := range video.RequestedFormats {
			URLs = append(URLs, format.URL, format.URL+"&video_id="+video.ID)
		}
	}

	//URLs = append(URLs, subtitleURLs...)

	return URLs, output, nil
}

func FindPath() (string, bool) {
	path, err := exec.LookPath("yt-dlp")
	if err != nil {
		return "", false
	}
	return path, true
}
