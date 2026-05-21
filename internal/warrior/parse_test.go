package warrior

import (
	"testing"

	"github.com/jordanm/atw-dashboard/internal/hub"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		line string
		want hub.Status
	}{
		{
			name: "out of date triggers error",
			line: "Project code is out of date and needs to be upgraded; aborting.",
			want: hub.Status{Error: true},
		},
		{
			name: "no http response triggers error",
			line: "No HTTP response received from tracker",
			want: hub.Status{Error: true},
		},
		{
			name: "tracker malfunction triggers error",
			line: "The tracker has probably malfunctioned. Will retry shortly.",
			want: hub.Status{Error: true},
		},
		{
			name: "kB/s triggers uploading",
			line: "Uploading at 250 kB/s",
			want: hub.Status{Uploading: true},
		},
		{
			name: "rate limiting triggers throttle",
			line: "Tracker rate limiting (sleep 60)",
			want: hub.Status{Throttle: true},
		},
		{
			name: "uploading and throttle simultaneously",
			line: "Tracker rate limiting; current 50 kB/s",
			want: hub.Status{Uploading: true, Throttle: true},
		},
		{
			name: "benign line",
			line: "Downloading https://example.com/page",
			want: hub.Status{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.line)
			if got != tt.want {
				t.Errorf("Classify(%q) = %+v, want %+v", tt.line, got, tt.want)
			}
		})
	}
}
