package leaderboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/jordanm/atw-dashboard/internal/hub"
)

const baseURL = "https://legacy-api.arpa.li"

type rawStats struct {
	Downloaders     []string           `json:"downloaders"`
	DownloaderBytes map[string]float64 `json:"downloader_bytes"`
	DownloaderCount map[string]uint64  `json:"downloader_count"`
}

type Poller struct {
	hub      *hub.Hub
	nickname string
	interval time.Duration
	client   *http.Client
}

func New(h *hub.Hub, nickname string, interval time.Duration) *Poller {
	return &Poller{
		hub:      h,
		nickname: nickname,
		interval: interval,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *Poller) Run(ctx context.Context) {
	// Run once on a short delay so the first refresh doesn't wait the full interval.
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			p.refreshAll(ctx)
			timer.Reset(p.interval)
		}
	}
}

func (p *Poller) refreshAll(ctx context.Context) {
	projects := p.hub.ActiveProjects()
	for _, project := range projects {
		stats, err := p.fetchProject(ctx, project)
		if err != nil {
			log.Printf("leaderboard %s: %v", project, err)
			continue
		}
		p.hub.SetLeaderboard(project, stats)
	}
}

func (p *Poller) fetchProject(ctx context.Context, project string) (*hub.ProjectStats, error) {
	url := fmt.Sprintf("%s/%s/stats.json", baseURL, project)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", resp.Status)
	}
	var raw rawStats
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	// Build a sorted ranking by bytes descending to determine position.
	type entry struct {
		name  string
		bytes float64
	}
	ranked := make([]entry, 0, len(raw.DownloaderBytes))
	for name, bytes := range raw.DownloaderBytes {
		ranked = append(ranked, entry{name, bytes})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].bytes > ranked[j].bytes
	})

	stats := &hub.ProjectStats{
		Project: project,
		Total:   len(raw.Downloaders),
	}
	for i, e := range ranked {
		if e.name == p.nickname {
			stats.Position = i + 1
			stats.Bytes = uint64(e.bytes)
			stats.Items = raw.DownloaderCount[e.name]
			break
		}
	}

	return stats, nil
}
