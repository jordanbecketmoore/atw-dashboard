package warrior

import (
	"strings"

	"github.com/jordanm/atw-dashboard/internal/hub"
)

// Classify mirrors the regex logic at the old assets/js/anwarConnection.js:72-96.
// Status flags are stateful on the warrior, but each item.output line replaces
// the previous flag set (matching the JS behavior of add/remove on every line).
func Classify(line string) hub.Status {
	var s hub.Status
	if strings.HasPrefix(line, "Project code is out of date and needs to be upgraded") ||
		strings.HasPrefix(line, "No HTTP response") ||
		strings.Contains(line, "The tracker has probably malfunctioned.") {
		s.Error = true
	}
	if strings.Contains(line, "kB/s") {
		s.Uploading = true
	}
	if strings.Contains(line, "Tracker rate limiting") {
		s.Throttle = true
	}
	return s
}
