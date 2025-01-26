package truthsocial

import (
	"encoding/json"
	"time"

	"github.com/internetarchive/Zeno/pkg/models"
)

type Status struct {
	ID                 string    `json:"id"`
	CreatedAt          time.Time `json:"created_at"`
	InReplyToID        any       `json:"in_reply_to_id"`
	QuoteID            any       `json:"quote_id"`
	InReplyToAccountID any       `json:"in_reply_to_account_id"`
	Sensitive          bool      `json:"sensitive"`
	SpoilerText        string    `json:"spoiler_text"`
	Visibility         string    `json:"visibility"`
	Language           string    `json:"language"`
	URI                string    `json:"uri"`
	URL                string    `json:"url"`
	Content            string    `json:"content"`
	Account            struct {
		ID                         string    `json:"id"`
		Username                   string    `json:"username"`
		Acct                       string    `json:"acct"`
		DisplayName                string    `json:"display_name"`
		Locked                     bool      `json:"locked"`
		Bot                        bool      `json:"bot"`
		Discoverable               bool      `json:"discoverable"`
		Group                      bool      `json:"group"`
		CreatedAt                  time.Time `json:"created_at"`
		Note                       string    `json:"note"`
		URL                        string    `json:"url"`
		Avatar                     string    `json:"avatar"`
		AvatarStatic               string    `json:"avatar_static"`
		Header                     string    `json:"header"`
		HeaderStatic               string    `json:"header_static"`
		FollowersCount             int       `json:"followers_count"`
		FollowingCount             int       `json:"following_count"`
		StatusesCount              int       `json:"statuses_count"`
		LastStatusAt               string    `json:"last_status_at"`
		Verified                   bool      `json:"verified"`
		Location                   string    `json:"location"`
		Website                    string    `json:"website"`
		UnauthVisibility           bool      `json:"unauth_visibility"`
		ChatsOnboarded             bool      `json:"chats_onboarded"`
		FeedsOnboarded             bool      `json:"feeds_onboarded"`
		AcceptingMessages          bool      `json:"accepting_messages"`
		ShowNonmemberGroupStatuses any       `json:"show_nonmember_group_statuses"`
		Emojis                     []any     `json:"emojis"`
		Fields                     []any     `json:"fields"`
		TvOnboarded                bool      `json:"tv_onboarded"`
		TvAccount                  bool      `json:"tv_account"`
	} `json:"account"`
	MediaAttachments []struct {
		ID               string `json:"id"`
		Type             string `json:"type"`
		URL              string `json:"url"`
		PreviewURL       string `json:"preview_url"`
		ExternalVideoID  string `json:"external_video_id"`
		RemoteURL        any    `json:"remote_url"`
		PreviewRemoteURL any    `json:"preview_remote_url"`
		TextURL          any    `json:"text_url"`
		Meta             struct {
			Colors struct {
				Background string `json:"background"`
				Foreground string `json:"foreground"`
				Accent     string `json:"accent"`
			} `json:"colors"`
			Original struct {
				Width     int     `json:"width"`
				Height    int     `json:"height"`
				FrameRate string  `json:"frame_rate"`
				Duration  float64 `json:"duration"`
				Bitrate   int     `json:"bitrate"`
			} `json:"original"`
			Small struct {
				Width  int     `json:"width"`
				Height int     `json:"height"`
				Size   string  `json:"size"`
				Aspect float64 `json:"aspect"`
			} `json:"small"`
		} `json:"meta"`
		Description any    `json:"description"`
		Blurhash    string `json:"blurhash"`
		Processing  string `json:"processing"`
	} `json:"media_attachments"`
	Mentions []struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		URL      string `json:"url"`
		Acct     string `json:"acct"`
	} `json:"mentions"`
	Tags            []any `json:"tags"`
	Card            any   `json:"card"`
	Group           any   `json:"group"`
	Quote           any   `json:"quote"`
	InReplyTo       any   `json:"in_reply_to"`
	Reblog          any   `json:"reblog"`
	Sponsored       bool  `json:"sponsored"`
	RepliesCount    int   `json:"replies_count"`
	ReblogsCount    int   `json:"reblogs_count"`
	FavouritesCount int   `json:"favourites_count"`
	Favourited      bool  `json:"favourited"`
	Reblogged       bool  `json:"reblogged"`
	Muted           bool  `json:"muted"`
	Pinned          bool  `json:"pinned"`
	Bookmarked      bool  `json:"bookmarked"`
	Poll            any   `json:"poll"`
	Emojis          []any `json:"emojis"`
}

func IsStatusesURL(URL *models.URL) bool {
	return statusesRegex.MatchString(URL.String())
}

func GenerateVideoURLsFromStatusesAPI(URL *models.URL) (assets []*models.URL, err error) {
	defer URL.RewindBody()

	decoder := json.NewDecoder(URL.GetBody())
	status := &Status{}

	if err := decoder.Decode(status); err != nil {
		return nil, err
	}

	// Generate the video URLs
	for _, mediaAttachment := range status.MediaAttachments {
		if mediaAttachment.ExternalVideoID != "" {
			assets = append(assets, &models.URL{
				Raw: "https://truthsocial.com/api/v1/truth/videos/" + mediaAttachment.ExternalVideoID,
			})
		}
	}

	return assets, nil
}
