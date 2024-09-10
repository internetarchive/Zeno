package ytdlp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func GetJSON(port int) (URLs []string, rawJSON string, HTTPHeaders HTTPHeaders, err error) {
	// Prepare the command
	cmd := exec.Command("yt-dlp", "--dump-json", "http://localhost:"+strconv.Itoa(port))

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
				URLs = append(URLs, format.URL, format.URL+"&video_id="+video.ID)
			}
		}
	}

	// Get all dubbed audio URLs
	for _, audio := range video.Formats {
		if strings.Contains(audio.FormatNote, "dubbed") {
			URLs = append(URLs, audio.URL, audio.URL+"&video_id="+video.ID)
		}
	}

	// write output to a .json file (debug)
	// err = ioutil.WriteFile("output.json", []byte(output), 0644)
	// if err != nil {
	// 	return nil, rawJSON, HTTPHeaders, fmt.Errorf("error writing output.json: %v", err)
	// }

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
