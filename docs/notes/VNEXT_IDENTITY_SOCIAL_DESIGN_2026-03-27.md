# Agent-Native IM VNext Identity, Social Graph, and Public Bot Access Design

Date: 2026-03-27
Status: Draft for implementation
Scope: `agent-native-im`, `agent-native-im-web`

## 1. Background

The current platform has completed the stability release, but the next feature cycle still lacks a coherent model for:

- unified external identity for humans and bots
- bot-specific external IDs and naming rules
- friend relationships between users and bots
- public or semi-public bot access before friendship
- frontend information architecture for chats, friends, bots, and groups

Today the backend still uses numeric database primary keys as canonical row identity. Public IDs are hydrated into `metadata.public_id` and exposed as `public_id`, but they are not first-class columns. There is no dedicated friend/contact model. There is also no policy layer for "this bot accepts conversations from non-friends" or "this bot is public but password-protected".

This document merges the following issue themes into one implementation direction:

- `agent-native-im #32` Human and bot default IDs unified as UUID
- `agent-native-im #31` bot creation must set an ID with `bot_` prefix
- `agent-native-im #30` users and bots can add each other as friends
- `agent-native-im #24` public bot capability, including anonymous or external access
- `agent-native-im-web #50` add friend entry and friend list
- `agent-native-im-web #49` show bot friends and joined groups
- `agent-native-im-web #51` reconsider direct/group separation in the sidebar

## 2. Product Goals

### 2.1 Primary goals

- Give every entity a stable external identity that can be safely shown, copied, routed, and referenced across clients and APIs.
- Support a real social graph instead of overloading conversation membership as a substitute for friendship.
- Let bot owners control whether the bot can be contacted before friendship.
- Support customer-service style bots that can accept inbound contact from non-friends or anonymous visitors.

### 2.2 Non-goals for this phase

- End-to-end encryption for AI agent messaging
- full anonymous session system for all platform features
- removal of internal numeric IDs from the database

## 3. Core Decisions

## 3.1 Internal ID vs external ID

Decision:

- Keep numeric `id` as the internal database primary key.
- Introduce first-class external UUID columns for entities and conversations in a future migration.
- Treat external UUID as the canonical public identifier for APIs, UI display, invite payloads, logs, and copy/share actions.

Rationale:

- Internal numeric IDs are efficient and already woven through the codebase.
- Replacing all internals with UUID immediately would create unnecessary migration and compatibility risk.
- A first-class external UUID avoids continuing to hide public identity inside JSON metadata.

Resulting rule:

- Internal references may keep using numeric IDs during a transition period.
- New public-facing API surfaces should prefer UUIDs.
- Existing numeric-ID APIs may remain temporarily for backward compatibility, but should be marked legacy.
- Project requirement going forward:
  all external-facing APIs, links, copy/share surfaces, logs, and UI identity displays should progressively standardize on `public_id`.
  Internal numeric `id` remains the short-term database primary key and internal join key, but should not be treated as the long-term public identity contract.

## 3.2 `public_id` vs `bot_id`

Decision:

- `public_id` is the universal external UUID for every entity.
- `bot_id` is a separate, optional, human-readable bot handle.
- `bot_id` is only for bots.
- `bot_id` must start with `bot_`.

Examples:

- user `public_id`: `550e8400-e29b-41d4-a716-446655440000`
- bot `public_id`: `9a9c83f1-3d7f-4f4a-9272-3e8b42d47be3`
- bot `bot_id`: `bot_support_cn`
- bot `bot_id`: `bot_acme_customer_service`

Rejected option:

- Using `bot_<uuid>` as the bot's only public identifier.

Reason rejected:

- It conflates machine identity and human-readable addressing.
- It makes future renames or productized bot handles harder.
- It forces user and bot identity into different public formats for no strong benefit.

Resulting rule:

- Humans and bots both have a UUID `public_id`.
- Bots may additionally have a unique `bot_id` handle with `bot_` prefix.
- UI should display both when useful:
  - primary: display name
  - secondary: `bot_id` when present
  - technical copy field: UUID `public_id`

## 3.3 Friendship model

Decision:

- Implement a real bilateral friend graph.
- Relationship type for this phase is `friend`, not generic `follow`.
- Friending is symmetric after acceptance.

Entities allowed:

- user <-> user
- user <-> bot
- bot <-> bot

Key rule:

- Friendship is not required for all conversation creation.
- Friendship is one policy input, not the only access gate.

## 3.4 Public or non-friend bot access

Decision:

- A bot can declare whether it accepts inbound conversations from non-friends.
- This policy is independent from friendship.
- Public bot access supports three visibility modes in phase order:

1. private
2. platform_public
3. external_public

Definitions:

- `private`: only owner-approved flows, existing friends, or explicit conversation/invite participation
- `platform_public`: logged-in platform users can initiate chat even if not friends
- `external_public`: unauthenticated external visitors can access the bot through a controlled public entry flow

Optional protection:

- A public bot may require an access password.

Use case covered:

- customer service bot
- marketing/demo bot
- enterprise support entrypoint

## 4. Target Data Model

## 4.1 Entity fields

Add first-class columns over time:

- `public_id uuid not null unique`
- `bot_id text null unique`
- `discoverability text not null default 'private'`
- `allow_non_friend_chat boolean not null default false`
- `require_access_password boolean not null default false`
- `access_password_hash text null`

Constraints:

- `bot_id` must be null for non-bot entities
- `bot_id` must match `^bot_[a-z0-9_]{3,64}$`
- `bot_id` uniqueness should be case-insensitive
- `allow_non_friend_chat` can only be true for bot entities
- `require_access_password` can only be true when discoverability is not `private`

Metadata cleanup direction:

- stop storing canonical `public_id` in metadata after migration
- retain read compatibility for old rows during the transition

## 4.2 Friendship tables

Introduce:

- `friend_requests`
- `friendships`

### `friend_requests`

Suggested fields:

- `id`
- `requester_entity_id`
- `target_entity_id`
- `status` enum: `pending|accepted|rejected|canceled|blocked`
- `message`
- `created_at`
- `updated_at`
- `acted_at`

Constraints:

- requester != target
- one active pending request per ordered pair

### `friendships`

Suggested fields:

- `id`
- `entity_a_id`
- `entity_b_id`
- `created_at`
- `created_from_request_id`

Constraints:

- store canonical ordering `entity_a_id < entity_b_id`
- unique pair on `(entity_a_id, entity_b_id)`

## 4.3 Public access sessions

Phase 1 should not introduce full anonymous accounts.

Instead add a dedicated public access flow:

- `bot_access_links`
- optional ephemeral visitor session concept later

Suggested `bot_access_links` fields:

- `id`
- `bot_entity_id`
- `code`
- `password_hash`
- `expires_at`
- `max_uses`
- `used_count`
- `created_by_entity_id`
- `created_at`

This enables:

- shareable customer-service links
- optional password gate
- revocation and expiration

## 5. Permission Model

## 5.1 Can a user start a direct conversation with a bot?

Allowed if any one of these is true:

- user is already a friend of the bot
- user owns the bot
- bot policy allows non-friend chat and requester is an authenticated platform user
- requester enters through a valid public access link
- requester is admitted through a future anonymous visitor flow

Denied otherwise.

## 5.2 Can a bot start a direct conversation with a user?

Default:

- only if already friends

Optional future relaxation:

- owner-controlled proactive outreach policy

This is out of scope for the first implementation.

## 5.3 Can anonymous users access a bot?

Not by default.

Allowed only when:

- bot `discoverability = external_public`
- access link is valid
- password check passes if configured

## 6. API Direction

## 6.1 Identity APIs

Add or evolve:

- `GET /entities/public/:publicId`
- `GET /bots/by-bot-id/:botId`
- `POST /entities/:id/public-id/regenerate` only if truly needed, otherwise avoid

Create/update bot payloads should accept:

- `bot_id`
- `discoverability`
- `allow_non_friend_chat`
- `access_password`

## 6.2 Friend APIs

Add:

- `POST /friends/requests`
- `GET /friends/requests`
- `POST /friends/requests/:id/accept`
- `POST /friends/requests/:id/reject`
- `POST /friends/requests/:id/cancel`
- `GET /friends`
- `DELETE /friends/:entityId`

Behavior:

- accepting creates a symmetric friendship
- deleting removes the friendship pair

## 6.3 Bot public access APIs

Add:

- `POST /bots/:id/access-links`
- `GET /bots/:id/access-links`
- `DELETE /bot-access-links/:id`
- `GET /public/bots/:botId-or-publicId`
- `POST /public/bots/:botId-or-publicId/session`
- `POST /public/bots/:botId-or-publicId/messages`

Important note:

- Public access APIs should not reuse the full authenticated session model blindly.
- They need explicit rate limits, abuse protection, and narrower capability scopes.

## 7. Frontend Information Architecture

## 7.1 Sidebar structure

Do not first split only `direct` vs `group`.

Preferred sidebar structure:

- Chats
- Friends
- Bots
- Settings

Inside `Chats`:

- All
- Direct
- Groups

Reason:

- This scales once friend graph exists.
- It avoids doing one navigation refactor now and another later.

## 7.2 Bot detail page

Add sections:

- Profile
- Access
- Friends
- Groups
- Diagnostics

Access section should show:

- `public_id`
- `bot_id`
- discoverability mode
- whether non-friend chat is allowed
- whether password protection is enabled
- public access links

Friends section should show:

- friend count
- paginated list
- relationship status for current user

Groups section should show:

- conversations of type `group` and `channel`

## 7.3 User profile / settings

Show:

- UUID `public_id`
- copy button
- friend requests entry
- friends list entry

## 8. Migration Strategy

## 8.1 Database migration

Required in the next implementation cycle:

1. add `public_id` column to `entities`
2. backfill from `metadata.public_id` where present
3. generate UUID for rows missing one
4. add unique index
5. add `bot_id`, discoverability, and access policy columns
6. add friend tables

Important:

- do not remove old metadata-based hydration in the same release
- keep dual-read compatibility for at least one release

## 8.2 API transition

Phase API migration:

- phase A: continue returning numeric `id` and `public_id`
- phase B: accept both numeric id and UUID in selected routes
- phase C: introduce public-ID-first frontend routes and share links

### Current legacy numeric-ID surfaces to retire gradually

These are acceptable short-term, but should not define the long-term public contract:

- authenticated management routes that still address records by internal `:id`
- internal web app routes such as `/chat/:conversationId` and `/bots/:botId`
- friend-management payloads using `entity_id` and numeric target IDs
- conversation creation payloads using numeric `participant_ids`

Migration rule:

- do not block product iteration by rewriting all of these immediately
- do stop introducing new external-facing features that depend on numeric `id`
- when a route is touched for product work, prefer adding or promoting a `public_id`-based variant instead of expanding numeric-ID dependence

## 8.3 Historical data

No special migration is needed for friendships because the feature does not exist yet.

For entity identity:

- existing rows already have numeric IDs
- many rows already have metadata-backed public UUIDs
- these should be backfilled into the new physical column

For bots:

- existing bots without `bot_id` remain valid
- `bot_id` should be optional for legacy bots at first
- new bot creation should require `bot_id`
- old bots can be prompted to claim a `bot_id`

## 9. Implementation Phases

## Phase 1: Identity foundation

Backend:

- add entity `public_id` column and backfill
- add `bot_id` and validation rules
- expose `public_id` and `bot_id` in entity APIs

Frontend:

- display `public_id` and `bot_id`
- support bot creation with `bot_id`

Definition of done:

- all entities have durable UUIDs in a first-class column
- new bots can be created with validated `bot_id`

## Phase 2: Friend graph

Backend:

- implement friend requests and friendships
- add list/search endpoints

Frontend:

- add Friends entry in sidebar
- friend request flows
- friend list page
- bot detail shows friend list

Definition of done:

- users and bots can become friends
- friend graph is visible and manageable in UI

## Phase 3: Non-friend bot chat inside platform

Backend:

- add bot access policy fields
- permit authenticated non-friend chat when policy allows

Frontend:

- surface bot access settings to owners
- allow start-chat CTA even when not friends if policy allows

Definition of done:

- platform users can start conversations with opted-in bots before friendship

## Phase 4: External public bot access

Backend:

- add public access links and password checks
- add scoped public messaging APIs

Frontend:

- bot owner access-link management
- public bot landing page
- public chat entry flow

Definition of done:

- external visitors can access selected bots without full platform signup

## 10. Risks

- Mixing UUID migration, friend graph, and public bot access into one release is too risky.
- Public access introduces abuse, spam, and rate-limit concerns.
- `bot_id` naming policy is product-visible and hard to change later.
- Frontend should not overfit to numeric IDs once UUID rollout starts.

## 11. Recommended Next Work Order

1. Implement first-class entity `public_id` column and backfill migration.
2. Add `bot_id` with `bot_` prefix validation for new bots.
3. Implement friend requests and friendship graph.
4. Add sidebar Friends entry and bot detail social sections.
5. Add non-friend chat policy for bots inside authenticated platform flows.
6. Add external public bot links and password protection.

## 12. Open Questions

- Should `bot_id` be globally unique across all environments, or only per deployment?
- Should friendship acceptance by bots be manual, auto-approved by owner policy, or configurable?
- Should `platform_public` bots appear in a global discover page, or only be reachable by direct link/search?
- Should external public chat create a temporary anonymous conversation record or a regular conversation with a synthetic visitor identity?

## 13. Recommended answers for now

- `bot_id` should be unique per deployment.
- Bots should support configurable auto-accept for incoming friend requests.
- `platform_public` bots should be reachable by direct link and search, but not forced into a global marketplace yet.
- External public chat should start with a narrow public access session model, not synthetic permanent entities.
