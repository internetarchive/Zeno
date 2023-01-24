package cloudflarestream

import (
	"encoding/xml"
	"errors"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/CorentinB/warc"
)

type MPD struct {
	XMLName                   xml.Name `xml:"MPD"`
	Text                      string   `xml:",chardata"`
	Xmlns                     string   `xml:"xmlns,attr"`
	Profiles                  string   `xml:"profiles,attr"`
	Type                      string   `xml:"type,attr"`
	MediaPresentationDuration string   `xml:"mediaPresentationDuration,attr"`
	MinBufferTime             string   `xml:"minBufferTime,attr"`
	Period                    struct {
		Text          string `xml:",chardata"`
		ID            string `xml:"id,attr"`
		AdaptationSet []struct {
			Text             string `xml:",chardata"`
			ID               string `xml:"id,attr"`
			MimeType         string `xml:"mimeType,attr"`
			SegmentAlignment string `xml:"segmentAlignment,attr"`
			Lang             string `xml:"lang,attr"`
			Representation   []struct {
				Text                      string `xml:",chardata"`
				ID                        string `xml:"id,attr"`
				AudioSamplingRate         string `xml:"audioSamplingRate,attr"`
				Bandwidth                 string `xml:"bandwidth,attr"`
				Codecs                    string `xml:"codecs,attr"`
				FrameRate                 string `xml:"frameRate,attr"`
				Height                    string `xml:"height,attr"`
				Width                     string `xml:"width,attr"`
				AudioChannelConfiguration struct {
					Text        string `xml:",chardata"`
					SchemeIdUri string `xml:"schemeIdUri,attr"`
					Value       string `xml:"value,attr"`
				} `xml:"AudioChannelConfiguration"`
				SegmentTemplate struct {
					Text           string `xml:",chardata"`
					Duration       string `xml:"duration,attr"`
					Initialization string `xml:"initialization,attr"`
					Media          string `xml:"media,attr"`
					StartNumber    string `xml:"startNumber,attr"`
					Timescale      string `xml:"timescale,attr"`
				} `xml:"SegmentTemplate"`
			} `xml:"Representation"`
		} `xml:"AdaptationSet"`
	} `xml:"Period"`
}

func Get(URL url.URL, httpClient warc.CustomHTTPClient) (URLs []url.URL, err error) {
	var (
		mpd        MPD
		mpdURL     string
		xmlDecoder xml.Decoder
	)

	// Replace /watch with /manifest/video.mpd if the URL ends with /watch, else, raise an error
	if len(URL.String()) < 6 {
		return nil, errors.New("cloudflaresteam.Parse: URL too short")
	} else {
		if strings.HasSuffix(URL.String(), "/watch") {
			mpdURL = strings.Replace(URL.String(), "/watch", "/manifest/video.mpd", 1)
		} else {
			return nil, errors.New("cloudflaresteam.Parse: URL does not end with /watch")
		}
	}

	// Get the MPD file from the new URL
	resp, err := httpClient.Get(mpdURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Verify that the content-type is application/dash+xml and that the status code is 200
	if resp.StatusCode != 200 {
		return nil, errors.New("cloudflaresteam.Parse: status code is not 200")
	}

	if resp.Header.Get("Content-Type") != "application/dash+xml" {
		return nil, errors.New("cloudflaresteam.Parse: content-type is not application/dash+xml")
	}

	// Unmarshal the MPD file into a struct with the xml package
	xmlDecoder = *xml.NewDecoder(resp.Body)
	err = xmlDecoder.Decode(&mpd)
	if err != nil {
		return nil, err
	}

	// Convert mpd.MediaPresentationDuration which is in the iso 8601 format to a time.Duration
	duration := parseDuration(mpd.MediaPresentationDuration)

	// For each AdaptationSet tag in the MPD file, parse the duration and timescale from the SegmentTemplate tag
	for _, adaptationSet := range mpd.Period.AdaptationSet {
		for _, representation := range adaptationSet.Representation {
			var (
				segmentTemplate = representation.SegmentTemplate
				timescale       int
				segmentDuration int
			)

			// Get the init.mp4 from the initialization attribute and strip the ../../ from the beginning
			initURL := strings.Replace(segmentTemplate.Initialization, "../../", "", 1)
			URLs = append(URLs, url.URL{
				Scheme: URL.Scheme,
				Host:   URL.Host,
				Path:   initURL,
			})

			timescale, err = strconv.Atoi(segmentTemplate.Timescale)
			if err != nil {
				return nil, err
			}

			segmentDuration, err = strconv.Atoi(segmentTemplate.Duration)
			if err != nil {
				return nil, err
			}

			// Calculate the number of segments in the video
			segments := duration * timescale / segmentDuration

			// For each segment, create a new URL and append it to the return URLs
			for i := 0; i < segments; i++ {
				var (
					segmentURL = segmentTemplate.Media
					segmentNum = strconv.Itoa(i + 1)
				)

				segmentURL = strings.Replace(segmentURL, "$Number$", segmentNum, 1)

				// Strip out ../../ from the URL
				segmentURL = strings.Replace(segmentURL, "../../", "", -1)

				URLs = append(URLs, url.URL{
					Scheme: URL.Scheme,
					Host:   URL.Host,
					Path:   segmentURL,
				})
			}
		}
	}

	return URLs, err
}

func parseDuration(duration string) int {
	var (
		days, hours, minutes int
		seconds              float64
	)

	duration = strings.TrimPrefix(duration, "P")

	for {
		var idx = strings.IndexAny(duration, "DTHM")

		if idx == -1 {
			break
		}

		var num, _ = strconv.Atoi(duration[:idx])

		switch duration[idx] {
		case 'D':
			days = num
		case 'H':
			hours = num
		case 'M':
			minutes = num
		}

		duration = duration[idx+1:]
	}

	if strings.HasSuffix(duration, "S") {
		duration = strings.TrimSuffix(duration, "S")
		seconds, _ = strconv.ParseFloat(duration, 64)
		seconds = math.Ceil(seconds)
	}

	return days*24*60*60 + hours*60*60 + minutes*60 + int(seconds)
}
