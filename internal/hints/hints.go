package hints

import (
	"fmt"

	"github.com/facets-cloud/bugbuster/internal/scenario"
)

// NextHint returns the next unseen hint, its cost, and its index.
// Returns an error if all hints have been used.
func NextHint(allHints []scenario.Hint, usedIndices []int) (text string, cost int, index int, err error) {
	if len(allHints) == 0 {
		return "", 0, 0, fmt.Errorf("no hints available for this scenario")
	}

	usedSet := make(map[int]bool, len(usedIndices))
	for _, idx := range usedIndices {
		usedSet[idx] = true
	}

	for i, h := range allHints {
		if !usedSet[i] {
			return h.Text, h.Cost, i, nil
		}
	}

	return "", 0, 0, fmt.Errorf("all %d hints have been used", len(allHints))
}

// RemainingHints returns how many hints are left.
func RemainingHints(allHints []scenario.Hint, usedIndices []int) int {
	return len(allHints) - len(usedIndices)
}

// TotalCostUsed returns the total point cost of all used hints.
func TotalCostUsed(allHints []scenario.Hint, usedIndices []int) int {
	total := 0
	for _, idx := range usedIndices {
		if idx >= 0 && idx < len(allHints) {
			total += allHints[idx].Cost
		}
	}
	return total
}
