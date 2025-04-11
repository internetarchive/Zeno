package reasoncode

import (
	"slices"

	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/discarder/cloudflare"
)

var HookNotSet = "Hook not set"
var EmptyHookChain = "HookChain is empty"
var AllPassed = "All hooks passed, no need to discard"

// IsChallengePage checks if the response is a challenge page.
func IsChallengePage(reason string) bool {
	reasons := []string{
		cloudflare.ChallengeDetected,
	}
	return slices.Contains(reasons, reason)
}
