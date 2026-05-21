# Dashboard for Archive Team Warrior

![Screenshot of a dashboard with multiple line graphs for bandwidth](screenshots/screenshot.png)

A live dashboard for a fleet of Archive Team Warriors. Originally a pure
client-side SPA, this version splits into a Go backend and a static frontend.
The backend runs in the same Kubernetes cluster as the warriors and connects to
them over the cluster's internal network — so the warriors no longer need to be
publicly exposed. The frontend talks only to the backend over REST + SSE.

## Architecture

```
                            ┌───────────────────────────────┐
   public Ingress ───────► │  atw-dashboard (Go binary)     │
                            │                                │
                            │  GET /            embedded     │
                            │                   frontend     │
                            │  GET /api/state   snapshot     │
                            │  GET /events      SSE stream   │
                            │  GET /healthz                  │
                            └────────────────┬───────────────┘
                                             │ ws (cluster-internal)
                                             ▼
                            ┌───────────────────────────────┐
                            │  warriors (private Services)  │
                            └───────────────────────────────┘
```

- **Backend** (`cmd/server`, `internal/*`): one goroutine per warrior holds a
  SockJS-over-WebSocket connection to `ws://<warrior>/websocket`, parses
  `bandwidth`, `item.output`, `project.refresh` events into an in-memory state
  hub, and fans them out to browser clients via Server-Sent Events. A
  leaderboard poller hits `legacy-api.arpa.li` every 5 min for each active
  project and computes the operator's rank server-side.
- **Frontend** (`web/`): static HTML/JS embedded into the binary via
  `//go:embed`. On load, fetches `/api/state` for an initial render, then opens
  `EventSource('/events')` for live updates. No third-party JS dependencies
  (the third-party `smoothie.js` chart library is vendored locally).

## Configuration

The backend reads YAML from `-config` (default `/etc/atw-dashboard/config.yaml`).
See `config.example.yaml`:

```yaml
listen_addr: ":8080"
nickname: "your-nickname"          # operator nick used for leaderboard lookups
leaderboard_interval: 5m
warriors:
  - name: warrior-1
    url: http://warrior-1.warriors.svc.cluster.local:8001
  - name: warrior-2
    url: http://warrior-2.warriors.svc.cluster.local:8001
```

## Local development

```sh
make tidy
make test
make build           # produces bin/atw-dashboard
make run-local CONFIG=$PWD/config.yaml
```

Or with Docker:

```sh
make docker
make run CONFIG=$PWD/config.yaml
```

Open <http://localhost:8080>.

## Kubernetes deploy

Manifests in `deploy/k8s/`:

```sh
# edit deploy/k8s/configmap.yaml with your nickname and warrior URLs
kubectl apply -f deploy/k8s/
```

Notes:

- The hub is in-memory and warrior connections are stateful — keep `replicas: 1`.
- The Ingress example sets nginx annotations to disable proxy buffering and
  raise the read timeout so SSE connections survive.
- Warriors should be reachable only inside the cluster. Drop any public
  Ingress / LoadBalancer that previously fronted them.

## Theming

Styling lives in `web/assets/user/user.css`. Drop in one of the theme files in
`web/assets/user/themes/` (lcars-picard, lcars-tng, light, metro) by copying
its `user.css` over the default. Per-instance log lines are tagged with the
warrior name as a CSS class (`#console p:has(.warrior-1) { ... }`).
