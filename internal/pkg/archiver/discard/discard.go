package discard

import (
	"net/http"

	"github.com/CorentinB/warc"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/discarder/dc_cloudflare"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/discarder/dc_common"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/reasoncode"
)

// Builder is a struct that helps build multiple discard hooks into a single one.
type Builder struct {
	hooks []warc.DiscardHook
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) AddHook(hook warc.DiscardHook) *Builder {
	b.hooks = append(b.hooks, hook)
	return b
}

func (b *Builder) AddDefaultHooks() *Builder {
	b.AddHook(dc_cloudflare.ChallengePageHook)
	b.AddHook(dc_common.WARCDiscardStatusHook)
	return b
}

// Build creates the final discard hook by chaining all the added hooks.
func (b *Builder) Build() warc.DiscardHook {
	return func(resp *http.Response) (bool, string) {
		if len(b.hooks) == 0 {
			return false, reasoncode.EmptyHookChain
		}

		for _, hook := range b.hooks {
			discard, reason := hook(resp)
			if discard {
				return true, reason
			}
		}

		return false, reasoncode.AllPassed
	}
}
