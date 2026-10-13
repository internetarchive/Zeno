package reasoncode

import (
	"github.com/internetarchive/Zeno/v2/internal/pkg/archiver/discard/discarder/botblock"
)

var HookNotSet = "Hook not set"
var EmptyHookChain = "HookChain is empty"
var AllPassed = "All hooks passed, no need to discard"

// BotBlockService returns the bot-blocking service (Cloudflare, Akamai, etc.)
// that caused the discard, if any.
func BotBlockService(reason string) (string, bool) {
	return botblock.Service(reason)
}
