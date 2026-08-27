# Tostada Portal Design

## Overview

Tostada is a multi-tenant workspace platform that spawns and manages remote desktops, notebooks, and web apps. The portal is a Go + React web application that serves as the user-facing catalog and launcher, using JupyterHub as the backend for authentication orchestration, pod spawning, and proxy routing.

## Architecture

```
┌─────────────────────────────────┐
│         OIDC Provider           │
│    (Gitea / Keycloak / GitHub)  │
└──────┬──────────────┬───────────┘
       │              │
┌──────▼──────┐ ┌─────▼────────────────────┐
│   Tostada   │ │       JupyterHub         │
│   Portal    │─┤  (admin API + KubeSpawner│
│ (Go+React)  │ │   + named servers)       │
└──────┬──────┘ └─────┬────────────────────┘
       │              │
       │         ┌────▼─────────────────┐
       │         │  Per-user pods       │
       │         │  - jupyter (:8888)   │
       │         │  - kasmvnc (:6901)   │
       │         │  - xrdp (:3389)      │
       │         └──────────────────────┘
       │
┌──────▼──────────────────────┐
│  Shared guacamole-client    │
│  + guacd (one deployment)   │
│  (for xrdp connections)     │
└─────────────────────────────┘
```

### Workspace routing

| Type | Per-user pod contents | User connects via |
|---|---|---|
| Jupyter | jupyter-server | JupyterHub proxy → `:8888` |
| KasmVNC | KasmVNC desktop | JupyterHub proxy → `:6901` |
| xrdp | xrdp desktop only | Shared guacamole-client (JSON auth token) |

### Key decisions

- **JupyterHub is the spawner**, not Guacamole. Guacamole has no spawner — KubeSpawner fills that gap.
- **Shared OIDC** — portal and JupyterHub authenticate against the same provider. Portal uses a JupyterHub admin API token to manage servers on behalf of users.
- **Named servers** — users can run multiple workspaces concurrently. Each workspace is a JupyterHub named server.
- **Shared guacamole-client + guacd** — one deployment serves all xrdp users. Per-user xrdp pods are lightweight (just the desktop). JSON auth eliminates the need for PostgreSQL.
- **KasmVNC bypasses guacd** — KasmVNC removed raw VNC protocol support and speaks only WebSocket via its own web client. JupyterHub proxies directly.
- **Stock KubeSpawner** — no custom SpawnerClass. Workspace types are configured via `singleuser.cmd`, image, and port overrides. Pre-spawn hooks inject env vars.

## Components & User Flow

### User flow

1. User visits Tostada portal
2. OIDC login (shared provider with JupyterHub)
3. Dashboard: workspace catalog (card grid) + active sessions list
4. User clicks "Launch" on a workspace type
   - Portal calls JupyterHub API to spawn a named server
   - For Jupyter/KasmVNC: redirect to JupyterHub proxy URL
   - For xrdp: generate JSON auth token, redirect to shared guacamole-client
5. User clicks "Connect" on an active session → same routing logic
6. User clicks "Stop" on an active session → JupyterHub API to stop the named server

### Go backend routes

| Route | Purpose |
|---|---|
| `GET /` | Serve React SPA |
| `GET /api/auth/login` | OIDC login redirect |
| `GET /api/auth/callback` | OIDC callback |
| `GET /api/workspaces` | List available workspace types (from config) |
| `GET /api/sessions` | List user's active sessions (JupyterHub API) |
| `POST /api/sessions` | Launch a workspace (spawn named server) |
| `DELETE /api/sessions/:name` | Stop a workspace |
| `GET /api/sessions/:name/connect` | Get connection URL (JupyterHub proxy or guacamole-client with token) |

### React frontend

- **Dashboard** — card grid of workspace types + list of active sessions
- No "connect" page — clicking Connect redirects to the workspace URL

### Workspace type config

```yaml
workspaces:
  - name: jupyter
    displayName: Jupyter Notebook
    description: Python data science environment
    icon: notebook
    type: jupyterhub
    image: jupyter/minimal-notebook:latest
    port: 8888
    cmd: ["jupyterhub-singleuser"]

  - name: kasmvnc-ubuntu
    displayName: Ubuntu Desktop (KasmVNC)
    description: Full Ubuntu desktop in browser
    icon: desktop
    type: jupyterhub
    image: kasmweb/ubuntu-noble-desktop:1.16.1
    port: 6901
    cmd: ["/dockerstartup/kasm_default_profile.sh"]

  - name: xrdp-ubuntu
    displayName: Ubuntu Desktop (xRDP)
    description: Full Ubuntu desktop via RDP
    icon: desktop
    type: guacamole
    image: scottyhardy/docker-remote-desktop:latest
    port: 3389
    rdpCredentials:
      username: ubuntu
      password: ubuntu
```

The `type` field determines routing: `jupyterhub` redirects to JupyterHub's proxy, `guacamole` generates a JSON auth token and redirects to the shared guacamole-client.

## Project Structure

```
tostada/
├── README.md
├── cmd/
│   └── tostada/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Load workspace config + OIDC settings
│   ├── auth/
│   │   └── oidc.go              # OIDC login/callback handlers
│   ├── hub/
│   │   └── client.go            # JupyterHub API client (spawn, stop, list)
│   ├── guacamole/
│   │   └── token.go             # JSON auth token generation (AES-CBC + HMAC)
│   └── api/
│       ├── router.go            # HTTP routes
│       ├── workspaces.go        # GET /api/workspaces
│       └── sessions.go          # CRUD /api/sessions
├── web/                         # React frontend (Vite)
│   ├── package.json
│   ├── vite.config.ts
│   └── src/
│       ├── App.tsx
│       ├── pages/
│       │   └── Dashboard.tsx    # Card grid + active sessions
│       └── components/
│           ├── WorkspaceCard.tsx
│           └── SessionList.tsx
├── k8s/
│   ├── jupyterhub-values.yaml   # JupyterHub Helm values
│   ├── guacamole-values.yaml    # Shared guacamole deployment
│   └── tostada-values.yaml      # Portal deployment
├── config.yaml                  # Workspace type definitions
├── Dockerfile                   # Multi-stage: build React + Go binary
├── skaffold.yaml                # Build + deploy orchestration
└── Makefile
```

## Deployment

All deployments run in a **kind cluster** for development.

### Three deployments

1. **JupyterHub** — official Helm chart with GenericOAuthenticator, KubeSpawner, named servers enabled, admin API token for portal
2. **Shared guacamole-client + guacd** — single deployment, JSON auth enabled, no PostgreSQL
3. **Tostada portal** — Go binary serving React SPA + API, configured with OIDC credentials, JupyterHub API URL + admin token, guacamole JSON auth secret

### Spawn flow

```
Portal                          JupyterHub                    Kubernetes
  │                                │                              │
  │ POST /hub/api/users/<user>/    │                              │
  │      servers/<name>            │                              │
  │ (image, cmd, port, env)        │                              │
  │───────────────────────────────►│                              │
  │                                │  KubeSpawner creates pod     │
  │                                │─────────────────────────────►│
  │                                │                              │
  │  200 OK (spawning)             │                              │
  │◄───────────────────────────────│                              │
  │                                │                              │
  │ Poll until ready               │                              │
  │───────────────────────────────►│                              │
  │                                │                              │
  │ Ready → redirect user          │                              │
  │ (JupyterHub proxy URL          │                              │
  │  or guacamole-client URL)      │                              │
```

### Dev workflow

- `make up` — starts kind cluster, deploys JupyterHub + guacamole + portal via Skaffold
- `make dev` — Skaffold dev mode (hot reload)
- `make down` — tears it all down

## Known considerations

- **Per-user URL prefix**: every app behind JupyterHub's proxy must handle `/user/<name>/<server>/` path prefix. KasmVNC and Jupyter handle this natively; xrdp bypasses JupyterHub's proxy entirely via shared guacamole-client.
- **xrdp readiness**: guacd hangs if xrdp isn't ready. Health-check port 3389 before allowing connection.
- **`/dev/shm`**: xrdp desktop pods need `/dev/shm` mounted as `emptyDir` with `medium: Memory`.
- **Future: per-user quota**: multiple concurrent workspaces with quota enforcement (not in MVP).
