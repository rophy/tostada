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