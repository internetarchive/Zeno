package cmd

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/internetarchive/Zeno/v2/internal/pkg/config"
	"github.com/internetarchive/Zeno/v2/internal/pkg/controler"
	"github.com/internetarchive/Zeno/v2/internal/pkg/ui"
	"github.com/internetarchive/Zeno/v2/pkg/models"
	"github.com/spf13/cobra"
)

var getListCmd = &cobra.Command{
	Use:   "list [FILE|URL...]",
	Short: "Archive URLs from text file(s)",
	Long: `Archive URLs from one or more text files or URLs.
Each file should contain one URL per line.
Remote files (starting with http:// or https://) are supported.
Empty lines and lines starting with # are ignored.
Lines that are not valid URLs are skipped with a warning.`,
	Args: cobra.MinimumNArgs(1),
	PreRunE: func(_ *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("viper config is nil")
		}

		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		// Read URLs from all provided files
		for _, file := range args {
			var urls []string
			var err error

			if strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") {
				urls, err = readRemoteURLList(file)
			} else {
				urls, err = readLocalURLList(file)
			}

			if err != nil {
				return fmt.Errorf("error reading file %s: %w", file, err)
			}

			// Add URLs to config
			config.Get().InputSeeds = append(config.Get().InputSeeds, urls...)
		}

		if len(config.Get().InputSeeds) == 0 {
			return fmt.Errorf("no URLs found in provided files")
		}

		err := config.GenerateCrawlConfig()
		if err != nil {
			return err
		}

		if cfg.PyroscopeAddress != "" {
			err = startPyroscope()
			if err != nil {
				return err
			}
		}

		if cfg.SentryDSN != "" {
			err = startSentry()
			if err != nil {
				return err
			}
		}

		controler.Start()
		if config.Get().TUI {
			tui := ui.New()
			err := tui.Start()
			if err != nil {
				return fmt.Errorf("error starting TUI: %w", err)
			}
		} else {
			controler.WatchSignals()
		}
		return nil
	},
}

// readLocalURLList reads URLs from a local file
func readLocalURLList(file string) (urls []string, err error) {
	f, err := os.Open(file)
	if err != nil {
		return urls, err
	}
	defer f.Close()

	return readURLList(f, file)
}

// readRemoteURLList reads URLs from a remote file (http/https)
func readRemoteURLList(URL string) (urls []string, err error) {
	httpClient := &http.Client{
		Timeout: time.Second * 30,
	}

	req, err := http.NewRequest(http.MethodGet, URL, nil)
	if err != nil {
		return urls, err
	}

	// Set user agent, use default if not configured
	userAgent := config.Get().UserAgent
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (compatible; Zeno)"
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return urls, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return urls, fmt.Errorf("failed to download URL list: %s", resp.Status)
	}

	return readURLList(resp.Body, URL)
}

// readURLList reads one URL per line, skipping the empty lines, the comments and the lines that
// are not valid URLs: one invalid line (e.g. with a control character) must not stop the whole crawl.
func readURLList(r io.Reader, source string) (urls []string, err error) {
	scanner := bufio.NewScanner(r)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++

		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Same check as when the seeds are added to the queue
		if _, err := models.NewURL(line); err != nil {
			fmt.Fprintf(os.Stderr, "skipping invalid URL on line %d of %s: %v\n", lineNumber, source, err)
			continue
		}

		urls = append(urls, line)
	}

	return urls, scanner.Err()
}
