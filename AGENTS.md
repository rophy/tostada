## Deployment

- `make up` deploys everything (kind cluster + Helm chart via skaffold, with coverage instrumentation)
- `make down` tears down the cluster
- `make e2e-test` runs e2e tests in-cluster via Kubernetes Job and collects server coverage
- No `.env` or docker-compose needed — everything runs on localhost via kind NodePorts

## Development Policy

1. Always run `make unit-test` before committing. All tests must pass.
2. Test coverage must be above 80% for both Go and TypeScript before pushing. If coverage drops below 80%, add tests before pushing.
3. All errors must be fixed before pushing. There is no such thing as "pre-existing errors" — if you see a failing test or lint error, fix it regardless of when it was introduced.

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

The `tostada` binary includes CLI subcommands for device and user management. Use `kubectl exec` to manage devices in the running cluster.

```bash
# Find the tostada pod
kubectl --context kind-tostada -n tostada get pods -l app=tostada

# List devices
kubectl --context kind-tostada -n tostada exec <pod> -c tostada -- /tostada device list

# Add a device
kubectl --context kind-tostada -n tostada exec <pod> -c tostada -- /tostada device add <name> <display> <proto> <host> <port> <user> <pass>

# Remove a device
kubectl --context kind-tostada -n tostada exec <pod> -c tostada -- /tostada device remove <name>

# Grant/revoke user access
kubectl --context kind-tostada -n tostada exec <pod> -c tostada -- /tostada device grant <device> <username>
kubectl --context kind-tostada -n tostada exec <pod> -c tostada -- /tostada device revoke <device> <username>

# Import devices from YAML
kubectl --context kind-tostada -n tostada exec <pod> -c tostada -- /tostada device import <file>
```

The database connection is configured via the `DATABASE_DSN` environment variable (injected from the `tostada-postgresql` secret).