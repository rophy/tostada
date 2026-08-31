# Tostada

The platform that holds everything on top — a multi-tenant workspace platform that spawns and manages remote desktops, notebooks, and web apps from a single pane of glass.

## Vision

Tostada uses **JupyterHub** as its orchestration layer — handling authentication, spawning, and proxying — to serve diverse workspace types through a unified portal:

| Workspace Type | How It Connects |
|---|---|
| Jupyter Notebook | Native JupyterHub proxy |
| KasmVNC Desktop | JupyterHub proxies WebSocket directly (KasmVNC's own web client) |
| xrdp Desktop | guacd translates RDP → WebSocket via guacamole-common-js |
| VNC Desktop | guacd translates VNC → WebSocket via guacamole-common-js |
| SSH Terminal | guacd or xterm.js |

## Architecture

```
┌─────────────────────────────────────────────────┐
│                   Tostada Portal                │
│            (custom UI + guacamole-common-js)     │
└────────────────────┬────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────┐
│              JupyterHub + KubeSpawner            │
│          (auth · spawning · proxy routing)        │
└──┬──────────┬──────────┬──────────┬─────────────┘
   │          │          │          │
   ▼          ▼          ▼          ▼
┌──────┐ ┌────────┐ ┌────────┐ ┌────────┐
│Jupyter│ │KasmVNC │ │  xrdp  │ │  VNC   │
│  Pod  │ │  Pod   │ │  Pod   │ │  Pod   │
│       │ │  :6901 │ │  :3389 │ │  :5900 │
└──────┘ └────────┘ └──┬─────┘ └──┬─────┘
                        │          │
                     ┌──▼──────────▼──┐
                     │     guacd      │
                     │ (protocol GW)  │
                     └────────────────┘
```

## Key Design Decisions

- **JupyterHub is the control plane**, not Guacamole. Guacamole has no spawner — it only connects to existing machines. KubeSpawner fills that gap.
- **guacd is a protocol gateway only.** We use `guacamole-common-js` to embed the remote desktop client in our own UI — no default Guacamole webapp needed.
- **KasmVNC bypasses guacd entirely.** KasmVNC removed raw VNC (RFB) protocol support and speaks only WebSocket via its own web client.
- **Stock KubeSpawner, no custom SpawnerClass.** Override `singleuser.cmd` per workspace type and use pre-spawn hooks for env injection.
- **Per-user URL prefix handling** (`/user/<name>/`) is required for every workspace type behind JupyterHub's proxy.

## Components

- **JupyterHub** — multi-tenant auth, spawning (KubeSpawner), and reverse proxy
- **guacd** — Apache Guacamole's protocol proxy daemon (RDP/VNC → WebSocket)
- **guacamole-common-js** — JavaScript SDK for embedding Guacamole sessions in custom pages
- **KasmVNC** — containerized Linux desktops with a native web client
- **xrdp** — RDP server for full desktop environments, accessed via guacd

## Deployment

Local development runs on a kind cluster with docker-compose providing the reverse proxy and OIDC mock.

### Prerequisites

- [kind](https://kind.sigs.k8s.io/)
- [skaffold](https://skaffold.dev/)
- [Helm](https://helm.sh/)
- [Docker Compose](https://docs.docker.com/compose/)
- Go, Node.js (use [mise](https://mise.jdx.dev/) — see `mise.toml`)

### Setup

1. Copy `.env.example` to `.env` and adjust values:

```sh
cp .env.example .env
```

2. Build Helm chart dependencies:

```sh
helm dependency build charts/tostada
```

3. Deploy everything:

```sh
make up
```

This creates the kind cluster, starts docker-compose (nginx proxy + OIDC mock), generates Helm values from `.env`, and deploys via skaffold.

### What `make up` does

```
.env  ──envsubst──▶  values-local.yaml  ──▶  Helm chart (tostada + jupyterhub subchart)
                                                │
docker-compose (nginx + oidc-mock)              │
       │                                        ▼
       └──────▶  kind cluster  ◀────────  skaffold run
```

- **docker-compose nginx** — single entry point, proxies OIDC endpoints to oidc-mock and everything else to the kind cluster gateway
- **skaffold** — builds the tostada image and deploys the Helm chart
- **values-local.yaml** — auto-generated from `.env`, gitignored

### Available targets

```
make help
```

### Teardown

```sh
make down
```
