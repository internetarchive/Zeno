package discard

import (
	"net/http"

	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/discarder/cloudflare"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/discarder/contentlength"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/discarder/warcdiscardstatus"
	"github.com/internetarchive/Zeno/internal/pkg/archiver/discard/reasoncode"
	"github.com/internetarchive/Zeno/internal/pkg/config"
	warc "github.com/internetarchive/gowarc"
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
	b.AddHook(cloudflare.ChallengePageHook)
	b.AddHook(warcdiscardstatus.WARCDiscardStatusHook)
	if config.Get().MaxContentLengthMiB > 0 {
		b.AddHook(contentlength.ContentLengthHook)
	}
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
