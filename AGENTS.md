## Git Hooks

This project uses gitleaks for pre-commit secret scanning.

- Hook location: `.githooks/pre-commit`
- Config: `.gitleaks.toml`
- Activated via: `git config core.hooksPath .githooks`

Before making any commit, verify the hook is active:
```bash
git config core.hooksPath
```
If it does not return `.githooks`, run `git config core.hooksPath .githooks` and confirm with the user before proceeding.

## Git Push Policy

NEVER push to any remote without explicit user confirmation in that specific message. Prior pushes do NOT grant standing permission.

## Managing Devices

The `tostada-cli` binary is included in the tostada container image at `/tostada-cli`. Use `kubectl exec` to manage devices in the running cluster — do NOT build or copy the CLI manually.

```bash
# Find the tostada pod
kubectl --context kind-tostada -n tostada get pods -l app=tostada

# List devices
kubectl --context kind-tostada -n tostada exec <pod> -c tostada -- /tostada-cli device list

# Add a device
kubectl --context kind-tostada -n tostada exec <pod> -c tostada -- /tostada-cli device add <name> <display> <proto> <host> <port> <user> <pass>

# Remove a device
kubectl --context kind-tostada -n tostada exec <pod> -c tostada -- /tostada-cli device remove <name>

# Grant/revoke user access
kubectl --context kind-tostada -n tostada exec <pod> -c tostada -- /tostada-cli device grant <device> <username>
kubectl --context kind-tostada -n tostada exec <pod> -c tostada -- /tostada-cli device revoke <device> <username>

# Import devices from YAML
kubectl --context kind-tostada -n tostada exec <pod> -c tostada -- /tostada-cli device import <file>
```

The DB path defaults to `TOSTADA_DB=/data/tostada.db` (set in the Dockerfile).