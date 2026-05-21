package hub

import (
	"sync"
	"time"
)

type Status struct {
	Error     bool `json:"error"`
	Uploading bool `json:"uploading"`
	Throttle  bool `json:"throttle"`
}

type WarriorState struct {
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Connected   bool      `json:"connected"`
	Project     string    `json:"project"`
	Sending     uint64    `json:"sending"`
	Receiving   uint64    `json:"receiving"`
	Sent        uint64    `json:"sent"`
	Received    uint64    `json:"received"`
	Status      Status    `json:"status"`
	LastUpdated time.Time `json:"last_updated"`
}

type ProjectStats struct {
	Project  string `json:"project"`
	Bytes    uint64 `json:"bytes"`
	Items    uint64 `json:"items"`
	Position int    `json:"position"`
	Total    int    `json:"total"`
}

type Snapshot struct {
	Nickname    string                   `json:"nickname"`
	Warriors    []WarriorState           `json:"warriors"`
	Leaderboard map[string]*ProjectStats `json:"leaderboard"`
}

type Event struct {
	Type    string `json:"-"`
	Payload any    `json:"-"`
}

type BandwidthPayload struct {
	Name      string `json:"name"`
	Sending   uint64 `json:"sending"`
	Receiving uint64 `json:"receiving"`
	Sent      uint64 `json:"sent"`
	Received  uint64 `json:"received"`
}

type StatusPayload struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
}

type ProjectPayload struct {
	Name    string `json:"name"`
	Project string `json:"project"`
}

type ConsolePayload struct {
	Name string    `json:"name"`
	Line string    `json:"line"`
	Time time.Time `json:"time"`
}

type ConnectionPayload struct {
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
	Message   string `json:"message,omitempty"`
}

type LeaderboardPayload struct {
	Project string        `json:"project"`
	Stats   *ProjectStats `json:"stats"`
}

type Hub struct {
	mu          sync.RWMutex
	nickname    string
	warriors    map[string]*WarriorState
	leaderboard map[string]*ProjectStats
	subs        map[uint64]chan Event
	nextSubID   uint64
}

func New(nickname string) *Hub {
	return &Hub{
		nickname:    nickname,
		warriors:    make(map[string]*WarriorState),
		leaderboard: make(map[string]*ProjectStats),
		subs:        make(map[uint64]chan Event),
	}
}

func (h *Hub) Nickname() string { return h.nickname }

func (h *Hub) RegisterWarrior(name, url string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.warriors[name]; !ok {
		h.warriors[name] = &WarriorState{Name: name, URL: url}
	}
}

func (h *Hub) get(name string) *WarriorState {
	w := h.warriors[name]
	if w == nil {
		w = &WarriorState{Name: name}
		h.warriors[name] = w
	}
	return w
}

func (h *Hub) UpdateBandwidth(name string, sending, receiving, sent, received uint64) {
	h.mu.Lock()
	w := h.get(name)
	w.Sending = sending
	w.Receiving = receiving
	w.Sent = sent
	w.Received = received
	w.LastUpdated = time.Now()
	h.mu.Unlock()

	h.publish(Event{Type: "bandwidth", Payload: BandwidthPayload{
		Name: name, Sending: sending, Receiving: receiving, Sent: sent, Received: received,
	}})
}

func (h *Hub) SetConnected(name string, connected bool, message string) {
	h.mu.Lock()
	w := h.get(name)
	if w.Connected == connected {
		h.mu.Unlock()
		return
	}
	w.Connected = connected
	if !connected {
		w.Status = Status{}
		w.Sending = 0
		w.Receiving = 0
	}
	h.mu.Unlock()

	h.publish(Event{Type: "connection", Payload: ConnectionPayload{
		Name: name, Connected: connected, Message: message,
	}})
}

func (h *Hub) SetProject(name, project string) {
	h.mu.Lock()
	w := h.get(name)
	if w.Project == project {
		h.mu.Unlock()
		return
	}
	w.Project = project
	h.mu.Unlock()

	h.publish(Event{Type: "project", Payload: ProjectPayload{Name: name, Project: project}})
}

func (h *Hub) SetStatus(name string, s Status) {
	h.mu.Lock()
	w := h.get(name)
	if w.Status == s {
		h.mu.Unlock()
		return
	}
	w.Status = s
	h.mu.Unlock()

	h.publish(Event{Type: "status", Payload: StatusPayload{Name: name, Status: s}})
}

func (h *Hub) AppendConsole(name, line string) {
	h.publish(Event{Type: "console", Payload: ConsolePayload{
		Name: name, Line: line, Time: time.Now(),
	}})
}

func (h *Hub) SetLeaderboard(project string, stats *ProjectStats) {
	h.mu.Lock()
	h.leaderboard[project] = stats
	h.mu.Unlock()

	h.publish(Event{Type: "leaderboard", Payload: LeaderboardPayload{Project: project, Stats: stats}})
}

func (h *Hub) ActiveProjects() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, w := range h.warriors {
		if w.Project != "" && !seen[w.Project] {
			seen[w.Project] = true
			out = append(out, w.Project)
		}
	}
	return out
}

func (h *Hub) Snapshot() Snapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	warriors := make([]WarriorState, 0, len(h.warriors))
	for _, w := range h.warriors {
		warriors = append(warriors, *w)
	}
	lb := make(map[string]*ProjectStats, len(h.leaderboard))
	for k, v := range h.leaderboard {
		cp := *v
		lb[k] = &cp
	}
	return Snapshot{Nickname: h.nickname, Warriors: warriors, Leaderboard: lb}
}

func (h *Hub) Subscribe() (uint64, <-chan Event, func()) {
	ch := make(chan Event, 64)
	h.mu.Lock()
	id := h.nextSubID
	h.nextSubID++
	h.subs[id] = ch
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		if c, ok := h.subs[id]; ok {
			delete(h.subs, id)
			close(c)
		}
		h.mu.Unlock()
	}
	return id, ch, unsub
}

func (h *Hub) publish(e Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.subs {
		select {
		case ch <- e:
		default:
		}
	}
}
