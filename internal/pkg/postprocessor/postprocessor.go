package postprocessor

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/internetarchive/Zeno/internal/pkg/config"
	"github.com/internetarchive/Zeno/internal/pkg/controler/pause"
	"github.com/internetarchive/Zeno/internal/pkg/log"
	"github.com/internetarchive/Zeno/internal/pkg/stats"
	"github.com/internetarchive/Zeno/pkg/models"
)

type postprocessor struct {
	wg                  sync.WaitGroup
	ctx                 context.Context
	cancel              context.CancelFunc
	inputCh             chan *models.Item
	outputCh            chan *models.Item
	hqOutlinksProduceCh chan *models.Item
}

var (
	globalPostprocessor *postprocessor
	once                sync.Once
	logger              *log.FieldedLogger
)

// This functions starts the preprocessor responsible for preparing
// the seeds sent by the reactor for captures
func Start(inputChan, outputChan, hqOutlinksProduceChan chan *models.Item) error {
	logger = log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor",
	})

	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		globalPostprocessor = &postprocessor{
			ctx:                 ctx,
			cancel:              cancel,
			inputCh:             inputChan,
			outputCh:            outputChan,
			hqOutlinksProduceCh: hqOutlinksProduceChan,
		}
		logger.Debug("initialized")
		for i := 0; i < config.Get().WorkersCount; i++ {
			globalPostprocessor.wg.Add(1)
			go globalPostprocessor.worker(strconv.Itoa(i))
		}
		logger.Info("started")
	})

	if globalPostprocessor == nil {
		return ErrPostprocessorAlreadyInitialized
	}

	return nil
}

func Stop() {
	if globalPostprocessor != nil {
		globalPostprocessor.cancel()
		globalPostprocessor.wg.Wait()
		logger.Info("stopped")
	}
}

func (p *postprocessor) worker(workerID string) {
	defer p.wg.Done()
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.worker",
		"worker_id": workerID,
	})

	stats.PostprocessorRoutinesIncr()
	defer stats.PostprocessorRoutinesDecr()

	// Subscribe to the pause controler
	controlChans := pause.Subscribe()
	defer pause.Unsubscribe(controlChans)

	for {
		select {
		case <-p.ctx.Done():
			logger.Debug("shutting down")
			return
		case <-controlChans.PauseCh:
			logger.Debug("received pause event")
			controlChans.ResumeCh <- struct{}{}
			logger.Debug("received resume event")
		case seed, ok := <-p.inputCh:
			if ok {
				logger.Debug("received seed", "seed", seed.GetShortID())

				if err := seed.CheckConsistency(); err != nil {
					panic(fmt.Sprintf("seed consistency check failed with err: %s, seed id %s", err.Error(), seed.GetShortID()))
				}

				if seed.GetStatus() != models.ItemArchived && seed.GetStatus() != models.ItemGotRedirected && seed.GetStatus() != models.ItemGotChildren {
					logger.Debug("skipping seed", "seed", seed.GetShortID(), "depth", seed.GetDepth(), "hops", seed.GetURL().GetHops(), "status", seed.GetStatus())
				} else {
					outlinks := postprocess(workerID, seed)
					outlinks = p.sendToHQOutlinks(outlinks)

					for i := range outlinks {
						select {
						case <-p.ctx.Done():
							logger.Debug("aborting outlink feeding due to stop", "seed", outlinks[i].GetShortID())
							return
						case p.outputCh <- outlinks[i]:
							logger.Debug("sending outlink", "seed", outlinks[i].GetShortID())
						}
					}
				}

				closeBodies(seed)

				select {
				case <-p.ctx.Done():
					logger.Debug("aborting seed due to stop", "seed", seed.GetShortID())
					return
				case p.outputCh <- seed:
				}
			}
		}
	}
}

// If options UseHQ, HQOutlinks & HQOutlinksHopLimit are selected, send outlinks
// to a different HQ project and don't return them for further processing.
func (p *postprocessor) sendToHQOutlinks(outlinks []*models.Item) []*models.Item {
	if config.Get().UseHQ && config.Get().HQOutlinksProject != "" && config.Get().HQOutlinksHopLimit > 0 {
		var filtered []*models.Item
		for i := range outlinks {
			if outlinks[i].GetURL().GetHops() >= config.Get().HQOutlinksHopLimit {
				logger.Info("sending outlink to HQ", "project", config.Get().HQOutlinksProject, "hop", outlinks[i].GetURL().GetHops())
				p.hqOutlinksProduceCh <- outlinks[i]
			} else {
				logger.Info("keeping outlink in the current project")
				filtered = append(filtered, outlinks[i])
			}
		}
		return filtered
	} else {
		return outlinks
	}
}

func postprocess(workerID string, seed *models.Item) []*models.Item {
	logger := log.NewFieldedLogger(&log.Fields{
		"component": "postprocessor.postprocess",
		"worker_id": workerID,
	})

	outlinks := make([]*models.Item, 0)

	childs, err := seed.GetNodesAtLevel(seed.GetMaxDepth())
	if err != nil {
		logger.Error("unable to get nodes at level", "err", err.Error(), "seed_id", seed.GetShortID())
		panic(err)
	}

	for i := range childs {
		seedOutlinks := postprocessItem(childs[i])
		outlinks = append(outlinks, seedOutlinks...)
	}

	return outlinks
}

func closeBodies(seed *models.Item) {
	seed.Traverse(func(seed *models.Item) {
		seed.Close()
	})
}
