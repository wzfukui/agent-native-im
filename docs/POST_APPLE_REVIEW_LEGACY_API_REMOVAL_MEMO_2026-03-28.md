# Post-Review Memo: Legacy Inbox Polling API Removal

Date: 2026-03-28

Background:
The web client now prefers the aggregate inbox snapshot endpoint:

- `GET /api/v1/inbox/snapshot`

It replaces the old polling pattern that repeatedly called:

- `GET /api/v1/entities`
- `GET /api/v1/friends/requests?entity_id=...&direction=incoming&status=pending`
- `GET /api/v1/notifications?entity_id=...&limit=200`

The old endpoints are still kept as compatibility fallback during the Apple review window.

Status:
- New server endpoint is deployed.
- Web has switched to snapshot-first with fallback.
- Old endpoints are still live.

After Apple review succeeds, do this cleanup:

1. Remove web fallback logic in [api.ts](/Users/donaldford/code/SuperBody/dev/agent-native-im-web/src/lib/api.ts) and [AppLayout.tsx](/Users/donaldford/code/SuperBody/dev/agent-native-im-web/src/layouts/AppLayout.tsx).
2. Keep per-entity endpoints only for explicit scoped pages/actions if still needed.
3. Re-check [FriendsPage.tsx](/Users/donaldford/code/SuperBody/dev/agent-native-im-web/src/pages/FriendsPage.tsx) and [InboxPage.tsx](/Users/donaldford/code/SuperBody/dev/agent-native-im-web/src/pages/InboxPage.tsx) for any leftover entity polling.
4. Decide whether `GET /api/v1/entities` still needs to be called on app shell load.
5. If no longer needed by web runtime, deprecate or remove these server routes:
   - `GET /api/v1/friends/requests`
   - `GET /api/v1/notifications`
6. Add one explicit release note that the inbox badge / pending friend request badge now relies on snapshot aggregation.
7. Verify network panel after removal:
   - app shell startup should not fan out into per-bot inbox polling
   - focus refresh should hit snapshot once
   - inbox/friends badge counts should remain correct

Do not remove immediately:
- `POST /api/v1/notifications/:id/read`
- `POST /api/v1/notifications/read-all`
- `POST /api/v1/friends/requests/:id/accept`
- `POST /api/v1/friends/requests/:id/reject`
- `POST /api/v1/friends/requests/:id/cancel`

Reason:
These are still action endpoints, not polling endpoints.

Success condition:
- No periodic per-entity inbox polling remains in web
- Snapshot is the only app-shell source of inbox badge data
- Mobile/App review is complete and no rollback path is needed
