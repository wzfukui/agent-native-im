# ANI Product Baseline

Last updated: 2026-03-29

This document is the index for the current ANI product baseline across backend, web, mobile, and OpenClaw integration.

It does not restate every requirement. It defines where the current source of truth lives and how to interpret cross-surface differences.

## Scope

Repositories:

- `dev/agent-native-im`
- `dev/agent-native-im-web`
- `dev/agent-native-im-mobile`
- `dev/openclaw/extensions/ani`
- `dev/openclaw-ani-installer`

## Source Of Truth Model

### 1. Platform behavior

Primary source:

- [user-stories.md](/Users/donaldford/code/SuperBody/dev/agent-native-im/docs/user-stories.md)
- [test-cases.md](/Users/donaldford/code/SuperBody/dev/agent-native-im/test-cases.md)

This layer defines platform-level truth:

- entity identity contract (`public_id`, `bot_id`)
- conversation and attachment semantics
- inbox / friendship / bot access policy rules
- message lifecycle and protected file behavior
- OpenClaw onboarding endpoints and access pack behavior

### 2. Web and PWA behavior

Primary source:

- [user-stories.md](/Users/donaldford/code/SuperBody/dev/agent-native-im-web/docs/user-stories.md)
- [test-cases.md](/Users/donaldford/code/SuperBody/dev/agent-native-im-web/test-cases.md)

Web is the reference implementation for browser and desktop-style ANI behavior.

Important rule:

- desktop layout may split navigation more aggressively than compact/mobile surfaces

### 3. Mobile behavior

Primary source:

- [PRODUCT_PARITY_BASELINE.md](/Users/donaldford/code/SuperBody/dev/agent-native-im-mobile/docs/PRODUCT_PARITY_BASELINE.md)
- [USER_STORIES.md](/Users/donaldford/code/SuperBody/dev/agent-native-im-mobile/docs/USER_STORIES.md)
- [MOBILE_PARITY_TEST_CASES_2026-03-29.md](/Users/donaldford/code/SuperBody/dev/agent-native-im-mobile/docs/MOBILE_PARITY_TEST_CASES_2026-03-29.md)

Mobile is the compact-layout implementation of ANI, not a separate product.

Important rule:

- compact layout may unify surfaces that desktop keeps separate

### 4. OpenClaw ANI integration

Primary source:

- [README.md](/Users/donaldford/code/SuperBody/dev/openclaw/extensions/ani/README.md)
- [TEST_MATRIX.md](/Users/donaldford/code/SuperBody/dev/openclaw/extensions/ani/docs/TEST_MATRIX.md)
- [README.md](/Users/donaldford/code/SuperBody/dev/openclaw-ani-installer/README.md)

This layer defines:

- plugin runtime behavior
- tool surface
- installation and update path
- operator validation for ANI-connected OpenClaw nodes

## Layout Interpretation Rules

Use these rules before declaring a surface "out of parity."

### Desktop layout

Desktop may split navigation into dedicated top-level areas when that reduces unread ambiguity or improves high-density workflows.

Current accepted example:

- desktop web splits `Direct` and `Groups`

### Compact layout

Compact surfaces may unify related navigation when screen density is limited.

Current accepted example:

- mobile / compact chat surfaces unify direct and group conversations under `Chats`

This is a deliberate form-factor adaptation, not a parity failure.

## Current Cross-Surface Invariants

These rules should remain aligned everywhere:

- new bots require valid `bot_id`
- entities expose stable `public_id`
- ANI files are protected resources, not naked public URLs
- inbox remains a first-class system event surface
- friend-first direct chat flows remain available
- OpenClaw onboarding defaults to `openclaw-ani-installer`

## Document Governance

When ANI behavior changes:

1. update platform stories or platform test cases if the rule is platform-wide
2. update web or mobile product docs only if the behavior is surface-specific
3. update OpenClaw ANI docs only if install/runtime/tooling changes
4. avoid leaving the newest truth only in `_experience`

`_experience` remains useful for design notes, migration memos, and internal reasoning, but it should not be the only home of current product truth.
