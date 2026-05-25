# Archive Team Warrior Dashboard

> **Note:** This repository contains AI-generated code.

![Screenshot of a dashboard with multiple line graphs for bandwidth](screenshots/screenshot.png)

A live dashboard for a fleet of Archive Team Warriors. Originally a pure
client-side SPA, this version splits into a Go backend and a static frontend.
The backend runs in the same Kubernetes cluster as the warriors and connects to
them over the cluster's internal network — so the warriors no longer need to be
publicly exposed. The frontend talks only to the backend over REST + SSE.

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

## Docker Installation

A Docker compose stack is provided in `install/docker/`. 

```bash 
git clone git@github.com:jordanbecketmoore/atw-dashboard.git
cd atw-dashboard/install/docker
docker compose up -d
```

## Kubernetes Installation

A Helm chart is provided in `install/helmchart/` and published to GHCR as an OCI artifact.
It deploys both the dashboard (Deployment + Service + ConfigMap) and the warriors
themselves (one StatefulSet per project, fronted by headless Services on the
cluster-internal network).

**Install from OCI (recommended):**

```sh
helm install atw oci://ghcr.io/jordanbmoore/atw-dashboard --version <version> -f my-values.yaml
```

To see available versions, check the [releases page](https://github.com/jordanbmoore/atw-dashboard/releases).

**Install from local chart:**

```sh
helm install atw ./chart -f my-values.yaml
```

Minimal `values.yaml`:

```yaml
warriors:
  nickname: your-nickname
  projects:
    - name: usgovernment
      replicas: 2

dashboard:
  image:
    repository: jordanbmoore/atw-dashboard
    tag: latest
  httproute:
    enabled: true
    parentRefs:
      - name: my-gateway
        namespace: gateway
    hostnames:
      - atw.example.com
```

The chart generates the dashboard `config.yaml` from `warriors.projects`,
naming each warrior `<project>-<index>` and pointing at
`http://<project>-warrior-<index>:8001`.

Notes:

- The hub is in-memory and warrior connections are stateful — the dashboard
  Deployment is pinned to `replicas: 1` with a `Recreate` strategy.
- External exposure uses the Gateway API (`HTTPRoute`), gated by
  `dashboard.httproute.enabled`. Bring your own `Gateway`.
- Warrior Services are cluster-internal only — they are not exposed via the
  HTTPRoute.
## Architecture

```
                            ┌────────────────────────────────┐
   Gateway HTTPRoute ────►  │  atw-dashboard (Go binary)     │
                            │                                │
                            │  GET /            embedded     │
                            │                   frontend     │
                            │  GET /api/state   snapshot     │
                            │  GET /events      SSE stream   │
                            │  GET /healthz                  │
                            └────────────────┬───────────────┘
                                             │ ws (cluster-internal)
                                             ▼
                            ┌────────────────────────────────┐
                            │  warriors (private Services)   │
                            └────────────────────────────────┘
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

### Browser ↔ warrior data flow

The browser-to-warrior path is not a forwarded WebSocket. Two different
protocols are joined by an in-process pub/sub hub:

- **Browser ↔ server** is one-way HTTP Server-Sent Events. The browser opens
  an `EventSource` to `GET /events`; the server holds the response open and
  streams JSON events.
- **Server ↔ warrior** is an outbound, server-initiated WebSocket per warrior,
  SockJS-framed (`ws://<warrior>/000/{session}/websocket`), with reconnect and
  exponential backoff. A read loop unwraps SockJS frames and dispatches
  payloads to the hub.
- **Hub** fans each event out to every subscribed SSE client. There is no
  per-connection routing — every browser receives every warrior's events, and
  each payload carries a `name` field the frontend uses to render the right
  card.
- **Direction is strict**: warriors push, browsers receive. There is no
  bidirectional pump, so the browser cannot send anything back to a warrior
  through this channel.

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


## Theming

Styling lives in `web/assets/user/user.css`. Drop in one of the theme files in
`web/assets/user/themes/` (lcars-picard, lcars-tng, light, metro) by copying
its `user.css` over the default. Per-instance log lines are tagged with the
warrior name as a CSS class (`#console p:has(.warrior-1) { ... }`).
