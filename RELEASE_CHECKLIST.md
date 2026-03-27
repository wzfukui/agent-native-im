# Release Checklist (Backend)

## Pre-check
- [ ] Confirm branch is `main` and working tree clean
- [ ] Verify required env vars in target env (`JWT_SECRET`, `ADMIN_PASS`, `DATABASE_URL`)
- [ ] Confirm DB backup/snapshot exists

## Quality gate
- [ ] `go test ./...`
- [ ] Run migration SQL in order (`migrations/*_up.sql`)
- [ ] Confirm `000014_entity_public_id_bot_id.up.sql` applied successfully
- [ ] Confirm `000015_friendships_and_bot_access.up.sql` applied successfully
- [ ] Smoke API: `GET /api/v1/ping` and auth flow
- [ ] Verify new bot creation rejects missing/invalid `bot_id`
- [ ] Verify successful bot creation returns both `public_id` and `bot_id`
- [ ] Verify `/api/v1/entities/discover` returns active users and excludes private bots
- [ ] Verify friend request create/accept/remove flow for user-user and user-bot
- [ ] Verify direct chat rejects non-friend human targets
- [ ] Verify direct chat succeeds for bot targets with `allow_non_friend_chat = true`
- [ ] Verify profile and entity avatar updates normalize to stored `/files/...` values
- [ ] Verify repeated bot token rotation remains owner-only and revokes prior keys

## Deploy
- [ ] `go build -o agent-native-im ./cmd/server`
- [ ] `sudo systemctl stop agent-im`
- [ ] Copy binary to `/opt/agent-im/agent-native-im`
- [ ] `sudo systemctl start agent-im`
- [ ] `sudo systemctl is-active agent-im` returns `active`
- [ ] Reverse proxy explicitly upgrades `GET /api/v1/ws`
- [ ] Reverse proxy forwards `Sec-WebSocket-Protocol` and `Authorization` to backend

## Post-check
- [ ] Verify logs have no panic or repeated 5xx
- [ ] Check key endpoints:
  - [ ] `/api/v1/conversations/public/:publicId`
  - [ ] `/api/v1/entities/:id/diagnostics`
  - [ ] `/api/v1/invite/:code`
  - [ ] `/avatar-files/:filename`
- [ ] Confirm browser WebSocket handshake returns `101 Switching Protocols` on `/api/v1/ws`
- [ ] Rollback plan confirmed (last known good binary)
