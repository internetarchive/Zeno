package ytdlp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

func GetJSON(port int) (URLs []string, rawJSON string, HTTPHeaders map[string]string, err error) {
	HTTPHeaders = make(map[string]string)

	// Prepare the command
	cmd := exec.Command("yt-dlp", "--dump-json", "http://localhost:"+strconv.Itoa(port), "-f", "bv[protocol=https]+ba[protocol=https]")

	// Buffers to capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()
	if err != nil {
		return URLs, rawJSON, HTTPHeaders, fmt.Errorf("yt-dlp error: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()

	// Parse the output as a Video object
	var video Video
	err = json.Unmarshal([]byte(output), &video)
	if err != nil {
		return nil, rawJSON, HTTPHeaders, fmt.Errorf("error unmarshaling yt-dlp JSON: %v", err)
	}

	// Get all subtitles (not automatic captions)
	var subtitleURLs []string
	for _, subtitle := range video.Subtitles {
		for _, sub := range subtitle {
			subtitleURLs = append(subtitleURLs, sub.URL)
		}
	}

	// Get all thumbnail URLs
	for _, thumbnail := range video.Thumbnails {
		URLs = append(URLs, thumbnail.URL)
	}

	// Get the manifest URL for the best video & audio quality
	// Note: we do not archive live streams
	if !video.IsLive {
		if len(video.RequestedFormats) > 0 {
			HTTPHeaders = video.RequestedFormats[0].HTTPHeaders
			for _, format := range video.RequestedFormats {
				URLs = append(URLs, format.URL+"&video_id="+video.ID)
			}
		}
	}

	// Get the storyboards
	for _, format := range video.Formats {
		if format.FormatNote == "storyboard" {
			URLs = append(URLs, format.URL)
			for _, fragment := range format.Fragments {
				URLs = append(URLs, fragment.URL)
			}
		}
	}

	URLs = append(URLs, subtitleURLs...)

	return URLs, output, HTTPHeaders, nil
}

func FindPath() (string, bool) {
	path, err := exec.LookPath("yt-dlp")
	if err != nil {
		return "", false
	}
	return path, true
}
