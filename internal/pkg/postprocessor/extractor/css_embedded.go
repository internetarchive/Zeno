package extractor

import (
	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/pkg/models"
)

// IsEmbeddedCSS checks if the item is an embedded CSS.
// An embedded CSS is a CSS item that is linked from an HTML item.
func IsEmbeddedCSS(item *models.Item) bool {
	ok, _ := isEmbeddedCSSWithJump(item, 0)
	return ok
}

// Returns the number of @import jumps to reach the HTML item.
//
// for example:
//
//	CSS -> HTML: 0 @import jump
//	CSS -> CSS -> HTML: 1 @import jump
//	CSS -> CSS -> CSS -> HTML: 2 @import jump
//	CSS -> ItemGotRedirected -> CSS -> CSS -> HTML: still 2 @import jump (ignores the redirection items)
func GetEmbeddedCSSJump(item *models.Item) int {
	ok, atImportJump := isEmbeddedCSSWithJump(item, 0)
	if !ok {
		cssLogger.Warn("item is not an embedded CSS, returning 0 @import jump", "func", "GetEmbeddedCSSJump", "item_id", item.GetShortID())
		return 0
	}
	return atImportJump
}

// Recursively check if the item is an ItemGotRedirected, and if so, skip it and return the parent item.
func skipRedirectedItem(item *models.Item) *models.Item {
	if item == nil {
		return nil
	}

	if item.GetStatus() == models.ItemGotRedirected {
		return skipRedirectedItem(item.GetParent())
	}

	return item
}

func isEmbeddedCSSWithJump(item *models.Item, atImportJump int) (bool, int) {
	// case0: item is nil, or all the items in the chain are ItemGotRedirected: not ok
	// case1: !CSS: not ok
	// case2: CSS: not ok
	// case3: CSS -> !HTML: not ok
	// case4: CSS -> HTML: ok
	// case5: CSS -> CSS: check the parent item. if the final parent is HTML, then ok, else not ok
	//
	// Ignore any ItemGotRedirected (HTTP 30X) items, as they are not relevant for the embedded CSS @import jump.

	// step to the first non-redirected item
	base := skipRedirectedItem(item)
	if base == nil {
		return false, 0 // case0
	}

	if IsCSS(base.GetURL()) {
		parent := skipRedirectedItem(base.GetParent())
		if parent == nil {
			return false, 0 // case2: no parent item
		}

		if IsCSS(parent.GetURL()) {
			return isEmbeddedCSSWithJump(parent, atImportJump+1) // case5: recursively check parent items
		} else if IsHTML(parent.GetURL()) {
			return true, atImportJump // case4: parent is HTML, so this is an embedded CSS
		} else {
			return false, 0 // parent is not HTML or CSS, so this is not an embedded CSS
		}
	}

	return false, 0 // case1: not a CSS item
}

// Add the @import links to the item as children items if the @import jump is less than the config.MaxCSSJump.
func AddAtImportLinksToItemChild(item *models.Item, atImportLinks []*models.URL) {
	// fast check
	if len(atImportLinks) == 0 {
		return
	}

	if GetEmbeddedCSSJump(item) >= config.Get().MaxCSSJump {
		cssLogger.Warn("item is an embedded CSS with a @import jump more than --max-css-jump, discarding at_import_links", "item_id", item.GetShortID(), "max_jump", config.Get().MaxCSSJump)
	} else {
		for _, link := range atImportLinks {
			newURL := &models.URL{
				Raw:  link.Raw,
				Hops: item.GetURL().GetHops(),
			}

			newChild := models.NewItem(newURL, "")
			err := item.AddChild(newChild, models.ItemGotChildren)
			if err != nil {
				panic(err)
			}
		}
	}
}
