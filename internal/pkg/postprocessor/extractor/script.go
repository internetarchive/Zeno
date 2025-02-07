package extractor

import (
	"encoding/json"
	"strings"
)

func extractFromScriptContent(content string) (assets []string, err error) {
	jsonContent := strings.SplitAfterN(content, "=", 2)

	if len(jsonContent) > 1 {
		var (
			openSeagullCount   int
			closedSeagullCount int
			payloadEndPosition int
		)

		// figure out the end of the payload
		for pos, char := range jsonContent[1] {
			if char == '{' {
				openSeagullCount++
			} else if char == '}' {
				closedSeagullCount++
			} else {
				continue
			}

			if openSeagullCount > 0 {
				if openSeagullCount == closedSeagullCount {
					payloadEndPosition = pos
					break
				}
			}
		}

		if len(jsonContent[1]) > payloadEndPosition {
			URLsFromJSON, _, err := GetURLsFromJSON(json.NewDecoder(strings.NewReader(jsonContent[1][:payloadEndPosition+1])))
			if err != nil {
				return nil, err
			} else {
				assets = append(assets, URLsFromJSON...)
			}
		}
	}

	return assets, nil
}
