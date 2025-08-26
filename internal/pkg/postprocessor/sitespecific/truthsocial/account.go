package truthsocial

import (
	"encoding/json"
	"time"

	"github.com/internetarchive/Zeno/internal/pkg/postprocessor/extractor"
	"github.com/internetarchive/Zeno/pkg/models"
)

type account struct {
	ID                         string    `json:"id"`
	Username                   string    `json:"username"`
	Acct                       string    `json:"acct"`
	DisplayName                string    `json:"display_name"`
	Locked                     bool      `json:"locked"`
	Bot                        bool      `json:"bot"`
	Discoverable               any       `json:"discoverable"`
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
	AcceptingMessages          bool      `json:"accepting_messages"`
	ChatsOnboarded             bool      `json:"chats_onboarded"`
	FeedsOnboarded             bool      `json:"feeds_onboarded"`
	TvOnboarded                bool      `json:"tv_onboarded"`
	BookmarksOnboarded         bool      `json:"bookmarks_onboarded"`
	ShowNonmemberGroupStatuses bool      `json:"show_nonmember_group_statuses"`
	Pleroma                    struct {
		AcceptsChatMessages bool `json:"accepts_chat_messages"`
	} `json:"pleroma"`
	TvAccount                 bool  `json:"tv_account"`
	ReceiveOnlyFollowMentions bool  `json:"receive_only_follow_mentions"`
	Emojis                    []any `json:"emojis"`
	Fields                    []any `json:"fields"`
}

type TruthsocialAccountOutlinkExtractor struct{}

func (TruthsocialAccountOutlinkExtractor) Support(m extractor.Mode) bool {
	return m == extractor.ModeGeneral
}

func (TruthsocialAccountOutlinkExtractor) Match(URL *models.URL) bool {
	return usernameRegex.MatchString(URL.String())
}

func (TruthsocialAccountOutlinkExtractor) Extract(URL *models.URL) (outlinks []*models.URL, err error) {
	defer URL.RewindBody()

	decoder := json.NewDecoder(URL.GetBody())
	account := &account{}

	if err := decoder.Decode(account); err != nil {
		return nil, err
	}

	outlinks = append(outlinks,
		&models.URL{
			Raw: "https://truthsocial.com/api/v1/accounts/" + account.ID + "/statuses?exclude_replies=true&only_replies=false&with_muted=true",
		},
		&models.URL{
			Raw: "https://truthsocial.com/api/v1/accounts/" + account.ID + "/statuses?pinned=true&only_replies=false&with_muted=true",
		},
		&models.URL{
			Raw: "https://truthsocial.com/api/v1/accounts/" + account.ID + "/statuses?with_muted=true&only_media=true",
		},
	)

	return outlinks, nil
}

type TruthsocialAccountLookupOutlinkExtractor struct{}

func (TruthsocialAccountLookupOutlinkExtractor) Support(m extractor.Mode) bool {
	return m == extractor.ModeGeneral
}

func (TruthsocialAccountLookupOutlinkExtractor) Match(URL *models.URL) bool {
	return accountLookupRegex.MatchString(URL.String())
}

func (TruthsocialAccountLookupOutlinkExtractor) Extract(URL *models.URL) (outlinks []*models.URL, err error) {
	// Get the username from the URL
	username := usernameRegex.FindStringSubmatch(URL.String())
	if len(username) != 2 {
		return nil, nil
	}

	// Generate the outlinks URLs
	outlinks = append(outlinks, &models.URL{
		Raw: "https://truthsocial.com/api/v1/accounts/lookup?acct=" + username[1],
	})

	return outlinks, nil
}
