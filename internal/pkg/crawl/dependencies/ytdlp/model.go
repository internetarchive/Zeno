package ytdlp

type Video struct {
	IsLive           bool `json:"is_live"`
	RequestedFormats []struct {
		URL string `json:"url"`
	} `json:"requested_formats"`
	Thumbnails []struct {
		URL string `json:"url"`
	} `json:"thumbnails"`
}
