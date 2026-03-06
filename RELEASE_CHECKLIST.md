# Release Checklist (Backend)

## Pre-check
- [ ] Confirm branch is `main` and working tree clean
- [ ] Verify required env vars in target env (`JWT_SECRET`, `ADMIN_PASS`, `DATABASE_URL`)
- [ ] Confirm DB backup/snapshot exists

## Quality gate
- [ ] `go test ./...`
- [ ] Run migration SQL in order (`migrations/*_up.sql`)
- [ ] Smoke API: `GET /api/v1/ping` and auth flow

## Deploy
- [ ] `go build -o agent-native-im ./cmd/server`
- [ ] `sudo systemctl stop agent-im`
- [ ] Copy binary to `/opt/agent-im/agent-native-im`
- [ ] `sudo systemctl start agent-im`
- [ ] `sudo systemctl is-active agent-im` returns `active`

## Post-check
- [ ] Verify logs have no panic or repeated 5xx
- [ ] Check key endpoints:
  - [ ] `/api/v1/conversations/public/:publicId`
  - [ ] `/api/v1/entities/:id/diagnostics`
- [ ] Rollback plan confirmed (last known good binary)
