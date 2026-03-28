# ANI VNext Task Brief

> Date: 2026-03-27
> Scope: `dev/agent-native-im` + `dev/agent-native-im-web`
> Goal: close the current stability gap first, then leave the codebase, docs, tests, and release notes in a releasable state.

## Version Goal

This cycle focuses on turning several active production-facing issues into a coherent VNext delivery.

Priority order:

1. Stability and reliability
2. Documentation and acceptance coverage
3. Release readiness
4. Identity / social graph design follow-up for the next feature cycle

## In Scope

### Track A: Stability fixes

- Fix invite-link access when a logged-in user opens an invite in a new tab and relies on cookie session restore.
- Fix avatar update success followed by avatar resource `404`.
- Fix or harden stale-build refresh behavior when the app shows `A newer build is available`.
- Stop repeated refetching in conversation context cards.
- Investigate token regeneration `409` failures and harden behavior where possible.

### Track B: Verification and coverage

- Add or update unit tests for the new behavior.
- Add or update end-to-end coverage for the critical user flows.
- Update test case documentation for the affected flows.

### Track C: Product and release documentation

- Update user stories to reflect the corrected behavior.
- Update release notes / release checklist inputs where needed.
- Record implementation notes and any known residual risks.

## Explicitly Out of Scope

These remain valid VNext-following topics, but are not part of this implementation batch:

- full friend graph / contacts system
- external ID unification (`UUID` vs prefixed public IDs)
- sidebar information architecture redesign around chats / friends / groups / bots
- encrypted messaging to AI agents

Those require product and data-model design, not just incremental bug fixing.

## Delivery Targets

### Backend

- Invite- and auth-related APIs must work correctly for cookie-restored web sessions.
- Avatar URLs saved by profile and entity update flows must resolve through the stable avatar route.
- Token rotation should be investigated with tests and any conflict-prone behavior reduced or at least documented with a reproducible path.

### Web

- Reopened invite links should work without forcing a fresh login.
- Newly updated avatars should display reliably.
- Build-refresh UX should recover from stale service worker / cache state more reliably.
- Conversation context cards should stop unnecessary repeated network activity.

## Acceptance Criteria

### Invite flow

- A logged-in user can reopen `/join/:code` in a new tab and successfully load invite info.
- The flow works when auth is restored from cookie rather than an existing tab token.

### Avatar delivery

- After updating an avatar from the web app, the rendered avatar resolves without `404`.
- Existing `/files/...` avatar references remain backward-compatible.

### Build refresh

- When drift is detected, the refresh action clears stale service workers / caches and loads the current bundle.
- The refresh notice should not remain stuck after a successful reload path.

### Conversation context card

- The card should not repeatedly refetch memories/tasks during ordinary renders.
- It should still refresh when the underlying conversation changes meaningfully.

### Token regeneration

- Rotation remains owner-only.
- Rotation can be repeated without spurious conflict failures in covered scenarios.
- If a remaining `409` path exists, it is documented and reproducible.

## Required Artifacts Before Close

- Code changes in both repos as needed
- Unit tests
- End-to-end tests
- Updated `docs/user-stories.md`
- Updated `test-cases.md`
- Release note / release checklist updates

## Execution Order

1. Write and save this brief
2. Implement stability fixes
3. Add verification coverage
4. Update product docs and test docs
5. Run verification
6. Update release documentation
7. Final release-readiness summary
