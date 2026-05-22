package warrior

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/jordanm/atw-dashboard/internal/config"
	"github.com/jordanm/atw-dashboard/internal/hub"
)

type frame struct {
	EventName string          `json:"event_name"`
	Message   json.RawMessage `json:"message"`
}

type bandwidthMsg struct {
	Sending   float64 `json:"sending"`
	Receiving float64 `json:"receiving"`
	Sent      uint64  `json:"sent"`
	Received  uint64  `json:"received"`
}

type itemOutputMsg struct {
	Data string `json:"data"`
}

type projectRefreshMsg struct {
	Project struct {
		Title string `json:"title"`
	} `json:"project"`
}

// Run drives a single warrior connection with reconnect backoff. Returns when ctx is done.
func Run(ctx context.Context, w config.Warrior, cfg *config.Config, h *hub.Hub) {
	h.RegisterWarrior(w.Name, w.URL)

	baseWSURL, err := buildWSBaseURL(w.URL)
	if err != nil {
		log.Printf("warrior %s: bad URL %q: %v", w.Name, w.URL, err)
		return
	}

	backoff := cfg.ReconnectMin
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		// SockJS framed WebSocket transport: /<server>/<session>/websocket.
		// The raw /websocket endpoint silently stops delivering periodic
		// broadcasts (bandwidth, timestamp) after the on_open dump — the
		// framed transport is what the official sockjs JS client uses.
		wsURL := fmt.Sprintf("%s/000/%s/websocket", baseWSURL, randomSession())
		err := connectOnce(ctx, w.Name, wsURL, h)
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			log.Printf("warrior %s: connection ended: %v", w.Name, err)
		}
		h.SetConnected(w.Name, false, fmt.Sprintf("disconnected: %v", err))

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > cfg.ReconnectMax {
			backoff = cfg.ReconnectMax
		}
	}
}

func connectOnce(ctx context.Context, name, wsURL string, h *hub.Hub) error {
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)

	h.SetConnected(name, true, "connected")
	h.AppendConsole(name, "Connected.")

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		if len(data) == 0 {
			continue
		}
		preview := data
		if len(preview) > 16384 {
			preview = preview[:16384]
		}
		log.Printf("warrior %s: recv %d bytes: %s", name, len(data), preview)
		// SockJS framing on the /websocket endpoint: 'o' open, 'h' heartbeat,
		// 'c[code,reason]' close, 'a[...]' array of JSON-encoded message strings.
		switch data[0] {
		case 'o', 'h':
			continue
		case 'c':
			return fmt.Errorf("server closed: %s", string(data[1:]))
		case 'a':
			var msgs []string
			if err := json.Unmarshal(data[1:], &msgs); err != nil {
				log.Printf("warrior %s: unmarshal sockjs array failed: %v", name, err)
				continue
			}
			for _, m := range msgs {
				var f frame
				if err := json.Unmarshal([]byte(m), &f); err != nil {
					log.Printf("warrior %s: unmarshal frame failed: %v", name, err)
					continue
				}
				dispatch(name, f, h)
			}
		case '{':
			// raw JSON (non-SockJS) — fall back to direct unmarshal
			var f frame
			if err := json.Unmarshal(data, &f); err != nil {
				log.Printf("warrior %s: unmarshal raw frame failed: %v", name, err)
				continue
			}
			dispatch(name, f, h)
		default:
			log.Printf("warrior %s: unknown frame type %q", name, data[0])
		}
	}
}

func dispatch(name string, f frame, h *hub.Hub) {
	switch f.EventName {
	case "bandwidth":
		var b bandwidthMsg
		if err := json.Unmarshal(f.Message, &b); err != nil {
			log.Printf("warrior %s: bandwidth unmarshal failed: %v", name, err)
		} else {
			h.UpdateBandwidth(name, uint64(b.Sending), uint64(b.Receiving), b.Sent, b.Received)
		}
	case "item.output":
		var m itemOutputMsg
		if err := json.Unmarshal(f.Message, &m); err == nil && m.Data != "" {
			h.SetStatus(name, Classify(m.Data))
			h.AppendConsole(name, m.Data)
		}
	case "project.refresh":
		var p projectRefreshMsg
		if err := json.Unmarshal(f.Message, &p); err == nil && p.Project.Title != "" {
			h.SetProject(name, strings.ToLower(strings.ReplaceAll(p.Project.Title, " ", "")))
		}
	case "timestamp":
		// heartbeat, ignored
	}
}

// buildWSBaseURL converts a warrior HTTP URL into a ws://host[:port][/prefix]
// base, with any trailing slash stripped. Callers append the SockJS
// per-session transport suffix (`/<server>/<session>/websocket`).
func buildWSBaseURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http", "":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
		// already correct
	default:
		return "", fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}

func randomSession() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
