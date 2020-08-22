package crawl

import (
	"context"
	"net/url"

	log "github.com/sirupsen/logrus"
	"mvdan.cc/xurls/v2"

	"github.com/CorentinB/Zeno/pkg/queue"
	"github.com/CorentinB/Zeno/pkg/utils"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Capture capture a page and queue the outlinks
func (c *Crawl) Capture(item *queue.Item) (outlinks []url.URL, err error) {
	_ = log.WithFields(log.Fields{
		"status_code": nil,
		"hop":         item.Hop,
	})

	// Create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	// Log requests
	chromedp.ListenBrowser(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventLoadingFinished:
			log.Info(ev.RequestID.String())
		}
	})

	// Run task
	err = chromedp.Run(ctx,
		network.Enable(),
		chromedp.Navigate(item.URL.String()),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if c.MaxHops > 0 {
				// Extract outer HTML
				node, err := dom.GetDocument().Do(ctx)
				if err != nil {
					return err
				}
				str, err := dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)

				// Extract outlinks and dedupe them
				rxStrict := xurls.Strict()
				rawOutlinks := utils.DedupeStringSlice(rxStrict.FindAllString(str, -1))

				// Validate outlinks
				for _, outlink := range rawOutlinks {
					URL, err := url.Parse(outlink)
					if err != nil {
						continue
					}
					err = utils.ValidateURL(URL)
					if err != nil {
						continue
					}
					outlinks = append(outlinks, *URL)
				}

				return err
			}
		}),
	)
	if err != nil {
		log.Info(err.Error())
		return nil, err
	}

	return outlinks, nil
}
