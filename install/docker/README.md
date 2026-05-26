# atw-dashboard Docker install

## Overview

Two files work together to run the dashboard alongside Archive Team warriors:

- **`docker-compose.yml`** — defines the containers (dashboard, warriors, watchtower) and mounts `config.yaml` into the dashboard container at `/etc/atw-dashboard/config.yaml`
- **`config.yaml`** — tells the dashboard where to find each warrior and what nickname to display

## Configuration

### Nickname

Set your downloader nickname in both places so they match:

- `config.yaml` → `nickname`
- `docker-compose.yml` → `DOWNLOADER` env var on each warrior service

### Warriors

The `warriors` list in `config.yaml` must match the hostnames Docker assigns to the warrior containers. Docker names containers using the pattern `<project>-<service>-<replica>`, where the project name is set by `name:` at the top of `docker-compose.yml` (`atw` by default).

With the default compose file (project `atw`, services `telegram` and `usgovernment`, 2 replicas each), the generated hostnames are:

```
atw-telegram-1     → http://atw-telegram-1:8001
atw-telegram-2     → http://atw-telegram-2:8001
atw-usgovernment-1 → http://atw-usgovernment-1:8001
atw-usgovernment-2 → http://atw-usgovernment-2:8001
```

These are already set correctly in `config.yaml`. If you add, remove, or rename warrior services, update the `warriors` list to match.

### Adding warriors

To run more projects or replicas:

1. Add a new service block in `docker-compose.yml` (copy an existing warrior block, change `SELECTED_PROJECT` and the service name)
2. Add a corresponding entry to the `warriors` list in `config.yaml` with the matching hostname and port `8001`

## Running

```sh
docker compose up -d
```

The dashboard is available at [http://localhost:8080](http://localhost:8080). Watchtower will automatically pull updated warrior images every hour.
