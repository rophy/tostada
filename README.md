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

Runs on Kubernetes (kind cluster for dev). Deployed via Helm charts.

## Status

Early stage — gathering architecture insights from related projects, scaffolding underway.
