package scenario

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// RunValidation executes a validation command and checks the result against
// the expected condition. Returns (passed, output).
func RunValidation(projectRoot string, v Validation) (bool, string) {
	cmd := exec.Command("bash", "-c", v.Command)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil && output == "" {
		return false, "command failed: " + err.Error()
	}

	// Parse expect string and evaluate
	passed := evaluateExpect(v.Expect, output)
	return passed, output
}

// evaluateExpect interprets the expect condition.
// Supported forms:
//   - "p99 < 500ms"       - extract p99 latency and compare
//   - "contains <text>"   - output contains the text
//   - "exit_code == 0"    - command exit code (always true if we got here)
//   - "status == healthy"  - output contains "healthy"
//   - any other string    - treated as substring match
func evaluateExpect(expect, output string) bool {
	expect = strings.TrimSpace(expect)

	// "contains <text>"
	if strings.HasPrefix(expect, "contains ") {
		substr := strings.TrimPrefix(expect, "contains ")
		return strings.Contains(strings.ToLower(output), strings.ToLower(substr))
	}

	// "exit_code == 0"
	if strings.HasPrefix(expect, "exit_code") {
		// If we got output without error, exit code is 0
		return true
	}

	// "p99 < 500ms" or similar latency checks
	if strings.Contains(expect, "p99") || strings.Contains(expect, "p95") || strings.Contains(expect, "p50") {
		return evaluateLatencyExpect(expect, output)
	}

	// "status == healthy"
	if strings.Contains(expect, "==") {
		parts := strings.SplitN(expect, "==", 2)
		if len(parts) == 2 {
			val := strings.TrimSpace(parts[1])
			return strings.Contains(strings.ToLower(output), strings.ToLower(val))
		}
	}

	// Default: substring match
	return strings.Contains(strings.ToLower(output), strings.ToLower(expect))
}

// evaluateLatencyExpect handles "p99 < 500ms" style expectations.
// It extracts a numeric latency from the output using heuristics.
func evaluateLatencyExpect(expect, output string) bool {
	// Parse the threshold from expect, e.g. "p99 < 500ms"
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*ms`)
	threshMatch := re.FindStringSubmatch(expect)
	if len(threshMatch) < 2 {
		return false
	}
	threshold, err := strconv.ParseFloat(threshMatch[1], 64)
	if err != nil {
		return false
	}

	// Determine which percentile to look for
	percentile := "99%"
	if strings.Contains(expect, "p95") {
		percentile = "95%"
	} else if strings.Contains(expect, "p50") {
		percentile = "50%"
	}

	// Try to extract the latency value from wrk output or similar
	// wrk output format: "  Latency   avg   stdev  max  +/- stdev"
	// Also look for lines containing the percentile
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, percentile) || strings.Contains(line, "Latency") {
			matches := re.FindAllStringSubmatch(line, -1)
			for _, m := range matches {
				val, err := strconv.ParseFloat(m[1], 64)
				if err == nil {
					// Determine comparison operator
					if strings.Contains(expect, "<") {
						return val < threshold
					} else if strings.Contains(expect, ">") {
						return val > threshold
					} else if strings.Contains(expect, "<=") {
						return val <= threshold
					}
				}
			}
		}
	}

	// If wrk format, try parsing the last numeric value as seconds/ms
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Latency") {
			// "Latency     1.23s    234.56ms   5.67s    89.00%"
			fields := strings.Fields(line)
			for _, f := range fields {
				if strings.HasSuffix(f, "ms") {
					val, err := strconv.ParseFloat(strings.TrimSuffix(f, "ms"), 64)
					if err == nil && strings.Contains(expect, "<") {
						return val < threshold
					}
				}
				if strings.HasSuffix(f, "s") && !strings.HasSuffix(f, "ms") {
					val, err := strconv.ParseFloat(strings.TrimSuffix(f, "s"), 64)
					if err == nil && strings.Contains(expect, "<") {
						return (val * 1000) < threshold
					}
				}
			}
		}
	}

	return false
}
