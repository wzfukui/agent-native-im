# Agent-Native IM Platform Test Cases

## Version 2.4 - Comprehensive Test Suite

This document contains detailed test cases for all platform features, including recent improvements for session recovery, avatar delivery, build refresh behavior, context-card fetch discipline, task management, and entity identity (`public_id` / `bot_id`) enforcement.

---

## 1. Entity Identity Tests

### TC-IDENTITY-001: Bot Creation Requires Explicit Bot ID
**Priority:** High
**Component:** Entity API, Bot Creation UI

**Preconditions:**
- Authenticated user session

**Steps:**
1. Submit bot creation request without `bot_id`
2. Submit bot creation request with invalid `bot_id` like `support_bot`
3. Submit bot creation request with valid `bot_id` like `bot_support_cn`

**Expected Result:**
- Missing `bot_id` is rejected with `400`
- Invalid `bot_id` format is rejected with `400`
- Valid request succeeds with `201`

### TC-IDENTITY-002: Entity Public UUID Is Stable And Present
**Priority:** High
**Component:** Entity API

**Preconditions:**
- User or bot entity exists

**Steps:**
1. Create a new user or bot
2. Read entity via `/me`, `/entities`, or create response
3. Store returned `public_id`
4. Read the same entity again

**Expected Result:**
- `public_id` is a valid UUID
- `public_id` remains unchanged across reads
- Legacy metadata-backed rows are backfilled into the first-class identity field

### TC-IDENTITY-003: Bot Handle Is Exposed Separately From Display Name
**Priority:** Medium
**Component:** Entity API, Bot Detail UI

**Preconditions:**
- Bot created with `display_name = "Acme Support"` and `bot_id = "bot_acme_support"`

**Steps:**
1. View bot detail page
2. Inspect API response for the same bot

**Expected Result:**
- Display name remains human-readable
- `bot_id` is shown as a technical handle
- Internal numeric `id` remains available for transition compatibility

---

## 2. Message Deduplication Tests

## 2. Friendship And Bot Access Tests

### TC-FRIEND-001: Friend Request Lifecycle
**Priority:** High
**Component:** Friend APIs, Friends UI

**Preconditions:**
- Two active user accounts exist

**Steps:**
1. User A sends a friend request to User B
2. User B loads incoming requests
3. User B accepts the request
4. User A loads friend list
5. User A removes the friendship

**Expected Result:**
- Friend request is created in `pending`
- Incoming request is visible to User B
- Accepting creates a symmetric friendship
- Both sides see each other in `/friends`
- Removing friendship succeeds without deleting prior conversations

### TC-FRIEND-002: User Can Act For Owned Bot
**Priority:** High
**Component:** Friend APIs

**Preconditions:**
- Authenticated user owns Bot X
- Another user or bot exists as target

**Steps:**
1. User sends friend request with `source_entity_id = Bot X`
2. Target accepts the request
3. User fetches `/friends?entity_id=<botX>`

**Expected Result:**
- Request is authorized
- Friendship is recorded for Bot X, not the owning user
- Bot X friend list returns the target entity

### TC-FRIEND-003: Inbox Notification Lifecycle For Friend Requests
**Priority:** High
**Component:** Notification API, Inbox UI

**Preconditions:**
- Two active user accounts exist

**Steps:**
1. User A sends a friend request to User B
2. User B loads `/notifications?status=unread`
3. User B accepts the request
4. User A loads `/notifications?status=unread`
5. User A marks the notification as read

**Expected Result:**
- User B receives `friend.request.received`
- User A receives `friend.request.accepted`
- Marking read updates `status` and `read_at`
- Both sides appear in `/friends`

### TC-BOT-ACCESS-001: Direct User Chat Requires Friendship
**Priority:** High
**Component:** Conversation API

**Preconditions:**
- User A and User B are active
- No friendship exists between them

**Steps:**
1. User A attempts to create a direct conversation with User B
2. Create friendship between User A and User B
3. Retry direct conversation creation

**Expected Result:**
- First attempt is rejected with `403`
- Second attempt succeeds after friendship exists

### TC-BOT-ACCESS-002: Support Bot Allows Non-Friend Direct Chat
**Priority:** High
**Component:** Entity Policy, Conversation API

**Preconditions:**
- User A is not friends with Bot X
- Bot X is configured with `discoverability = platform_public`
- Bot X has `allow_non_friend_chat = true`

**Steps:**
1. User A creates a direct conversation with Bot X

**Expected Result:**
- Request succeeds with `201`
- Conversation is created without a prior friendship

### TC-BOT-ACCESS-003: Private Bot Stays Hidden From Discoverable Search
**Priority:** Medium
**Component:** Discoverable Entity Search

**Preconditions:**
- Bot X exists with `discoverability = private`

**Steps:**
1. Search `/entities/discover?q=<bot_id>`

**Expected Result:**
- Bot X is not returned in results

### TC-BOT-ACCESS-004: External Public Bot Requires Password
**Priority:** High
**Component:** Public Bot Access

**Preconditions:**
- Bot X exists with `discoverability = external_public`
- Bot X has `require_access_password = true`
- Bot X has a valid public access password

**Steps:**
1. Load `/public/bots/<bot_id>`
2. Attempt to create a public session with the wrong password
3. Retry with the correct password

**Expected Result:**
- Step 2 fails with `401`
- Step 3 succeeds with `201`
- A temporary visitor session and direct conversation are created

### TC-BOT-ACCESS-005: Public Access Link Creates Guest Session
**Priority:** High
**Component:** Public Bot Access Links

**Preconditions:**
- Bot X exists and is externally public
- Owner created at least one access link for Bot X

**Steps:**
1. Open `/public/bots/<bot_id>?code=<link_code>`
2. Start a guest session
3. Send a message into the returned conversation

**Expected Result:**
- Public bot landing page loads
- Guest session is created successfully
- Guest can send a message into the direct conversation

## 3. Message Deduplication Tests

### TC-DEDUP-001: Prevent Duplicate Messages on Reconnection
**Priority:** High
**Component:** WebSocket Client, Message Manager

**Preconditions:**
- User connected to WebSocket
- Active conversation with message history

**Steps:**
1. Send message M1 from user
2. Receive message M2 from bot
3. Disconnect WebSocket (kill network)
4. Reconnect WebSocket
5. Verify no duplicate of M1 or M2 appears

**Expected Result:**
- Messages M1 and M2 appear exactly once
- No duplicates after reconnection
- Message order preserved

**Test Data:**
```javascript
const testMessage = {
  id: 12345,
  conversation_id: 1,
  layers: { summary: "Test message" }
}
```

---

### TC-DEDUP-002: Handle Out-of-Order Message Delivery
**Priority:** High
**Component:** Message Deduplication Manager

**Preconditions:**
- Message dedup manager initialized
- Conversation with sequential messages

**Steps:**
1. Receive message with ID 100
2. Receive message with ID 102 (out of order)
3. Receive message with ID 101 (delayed)
4. Check final message order

**Expected Result:**
- Messages displayed in order: 100, 101, 102
- Message 102 buffered until 101 arrives
- No messages lost or duplicated

---

### TC-DEDUP-003: Message ID Set Overflow Protection
**Priority:** Medium
**Component:** Message Deduplication Manager

**Preconditions:**
- Empty message ID set

**Steps:**
1. Add 1000 unique messages
2. Continue adding 100 more messages
3. Verify set size is trimmed
4. Verify old IDs are removed
5. Verify new IDs are retained

**Expected Result:**
- Set size stays within bounds (≤1000)
- FIFO removal of old IDs
- Recent messages always deduplicated

---

## 2. Accessibility Tests

### TC-A11Y-001: Keyboard Navigation Flow
**Priority:** High
**Component:** UI Navigation

**Preconditions:**
- Application loaded
- User not using mouse

**Steps:**
1. Press Tab to navigate through UI
2. Verify focus order: Header → Sidebar → Main → Footer
3. Press Shift+Tab to reverse
4. Press Enter on focusable elements
5. Press Escape to close modals

**Expected Result:**
- Logical tab order maintained
- All interactive elements reachable
- Visual focus indicators visible
- No keyboard traps

---

### TC-A11Y-002: Screen Reader Announcements
**Priority:** High
**Component:** ARIA Implementation

**Preconditions:**
- Screen reader enabled (NVDA/JAWS/VoiceOver)

**Steps:**
1. Navigate to bot list
2. Select a bot
3. Send a message
4. Receive bot response
5. Navigate to tasks

**Expected Result:**
- All elements announced correctly
- Role, state, and properties conveyed
- Dynamic updates announced
- No redundant announcements

**Verification Points:**
```html
<!-- Example ARIA labels to verify -->
<button aria-label="Start conversation with SuperBody bot">
<div role="status" aria-live="polite">Message sent</div>
<nav aria-label="Main navigation">
```

---

### TC-A11Y-003: Keyboard Shortcuts
**Priority:** Medium
**Component:** Keyboard Handler

**Preconditions:**
- Application loaded
- Focus in main area

**Steps:**
1. Press Cmd/Ctrl+K → Search opens
2. Press Cmd/Ctrl+1 → First tab selected
3. Press Cmd/Ctrl+N → New conversation
4. Press Cmd/Ctrl+Enter → Send message
5. Press Cmd/Ctrl+/ → Help menu

**Expected Result:**
- All shortcuts work as documented
- No conflicts with browser shortcuts
- Shortcuts respect focus context
- Help menu shows all shortcuts

---

### TC-A11Y-004: Focus Management in Modals
**Priority:** High
**Component:** Modal Components

**Preconditions:**
- Application loaded

**Steps:**
1. Open settings modal
2. Verify focus trapped in modal
3. Tab through all modal elements
4. Press Escape to close
5. Verify focus returns to trigger

**Expected Result:**
- Focus trapped within modal
- Can't tab to background elements
- Escape closes modal
- Focus returns to original element

---

## 3. Error Handling Tests

### TC-ERROR-001: Structured Error Response (v2.3+)
**Priority:** High
**Component:** API Client, Error Handler

**Preconditions:**
- API client initialized
- Invalid request prepared

**Steps:**
1. Send request with invalid data
2. Receive 400 error response
3. Parse error structure
4. Display error to user
5. Log error details

**Expected Result:**
```json
{
  "ok": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid priority value",
    "status": 400,
    "request_id": "req_abc123",
    "details": {
      "field": "priority",
      "allowed": ["low", "medium", "high"]
    }
  }
}
```

---

### TC-ERROR-002: Backward Compatibility with Legacy Errors

---

## 4. Stability Regression Tests

### TC-STABILITY-001: Invite Page Works After Cookie Session Restore
**Priority:** High
**Component:** Web Session Restore, Invite Flow

**Preconditions:**
- User has a valid auth cookie
- No tab-local `sessionStorage` token exists
- Invite code is valid

**Steps:**
1. Open a fresh browser tab
2. Navigate directly to `/join/<code>`
3. Allow the app to restore the session through `GET /api/v1/me`
4. Observe the invite page load
5. Confirm the invite request is sent without a synthetic placeholder bearer token

**Expected Result:**
- Invite page renders normally
- Conversation title appears
- User is not forced back to login
- Invite API request relies on cookie-backed auth, not `Bearer __cookie_session__`

### TC-STABILITY-002: Avatar Update Does Not Regress to 404
**Priority:** High
**Component:** Profile Update, Avatar Delivery

**Preconditions:**
- Authenticated user or bot owner
- Uploadable image file available

**Steps:**
1. Upload a new avatar through the web UI
2. Save the updated profile or bot details
3. Reload the page
4. Open the avatar resource URL from the rendered image

**Expected Result:**
- Save succeeds
- Avatar renders after reload
- Avatar URL resolves through `/avatar-files/...`
- No `404` is returned for the resulting avatar resource

### TC-STABILITY-003: Same-Commit Rebuild Does Not Trigger False Stale Build Warning
**Priority:** Medium
**Component:** Build Drift Detection

**Preconditions:**
- Current web bundle version and commit match the deployed bundle
- Build time differs between local and deployed metadata

**Steps:**
1. Load the app
2. Fetch `build-info.json`
3. Compare version, commit, and build time

**Expected Result:**
- No stale-build warning is shown when only build time differs
- Warning still appears if version or commit differs

### TC-STABILITY-004: Conversation Context Card Avoids Prompt-Only Refetches
**Priority:** High
**Component:** Conversation Context Card

**Preconditions:**
- Conversation contains memories and tasks
- Context card is visible

**Steps:**
1. Open a conversation with the context card
2. Let the initial card fetch complete
3. Trigger a prompt-only rerender without changing the conversation identity
4. Observe network activity

**Expected Result:**
- Initial memories/tasks fetch runs once
- Prompt-only rerender does not trigger duplicate `listMemories` / `listTasks` calls
- Existing context content stays visible

### TC-STABILITY-005: Repeated Token Rotation Remains Safe
**Priority:** High
**Component:** Bot Token Rotation

**Preconditions:**
- Bot exists and belongs to the authenticated user

**Steps:**
1. Rotate the bot token once
2. Confirm the new key works
3. Rotate the token again
4. Confirm the first rotated key no longer works
5. Confirm the latest key still works

**Expected Result:**
- Rotation succeeds without spurious conflict failures
- Previous permanent key is revoked
- Latest key remains valid
**Priority:** High
**Component:** Error Parser

**Preconditions:**
- Legacy backend returning old format

**Steps:**
1. Send request to legacy endpoint
2. Receive old error format: `{"error": "Something went wrong"}`
3. Parse with new error handler
4. Verify compatibility

**Expected Result:**
- Old format parsed correctly
- Error message extracted
- No crash or undefined behavior
- User sees meaningful error

---

### TC-ERROR-003: Rate Limit Handling
**Priority:** Medium
**Component:** API Client

**Preconditions:**
- Rate limit configured on backend

**Steps:**
1. Send requests rapidly
2. Trigger rate limit (429)
3. Parse retry-after header
4. Wait specified duration
5. Retry request

**Expected Result:**
- 429 error caught correctly
- Retry-after value extracted
- Automatic retry after delay
- User informed of rate limit

---

## 4. Task Management Tests

### TC-TASK-001: Create Task with Full Details
**Priority:** High
**Component:** Task API

**Preconditions:**
- Active conversation
- Authenticated user

**Steps:**
1. Create task with all fields:
   - Title: "Fix login bug"
   - Description: "Users can't login with GitHub"
   - Priority: "high"
   - Assignee: Bot ID 123
   - Due date: "2024-01-15T00:00:00Z"
2. Verify task created
3. Check all fields saved

**Expected Result:**
- Task created with unique ID
- All fields correctly stored
- Task appears in conversation
- Assignee notified

---

### TC-TASK-002: Task Status Lifecycle
**Priority:** High
**Component:** Task State Machine

**Preconditions:**
- Task in "pending" status

**Steps:**
1. Update task to "in_progress"
2. Verify status change
3. Update task to "done"
4. Verify completion
5. Attempt invalid transition

**Expected Result:**
- Status transitions successful
- Updated_at timestamp changes
- Invalid transitions rejected
- Audit trail maintained

---

### TC-TASK-003: Task Dependencies and Blocking
**Priority:** Medium
**Component:** Task Dependency System

**Preconditions:**
- Parent task T1 created
- Child task T2 with parent_task_id = T1

**Steps:**
1. Check T2.is_blocked → true
2. Try to start T2 → blocked
3. Complete T1
4. Check T2.is_blocked → false
5. Start T2 → success

**Expected Result:**
- Child blocked while parent incomplete
- Unblocked after parent completion
- Clear blocking indication in UI
- Dependency chain respected

---

### TC-TASK-004: Task Due Date and Overdue Detection
**Priority:** Medium
**Component:** Task Scheduler

**Preconditions:**
- Task with due_date in past

**Steps:**
1. Create task with past due date
2. Check is_overdue property
3. List overdue tasks
4. Update due date to future
5. Verify no longer overdue

**Expected Result:**
- Overdue tasks flagged correctly
- Timezone handling correct
- UI shows overdue indicator
- Notifications sent for overdue

---

## 5. Python SDK Tests

### TC-SDK-001: Bot Initialization and Connection
**Priority:** High
**Component:** Python SDK

**Preconditions:**
- Valid bot token
- Backend running

**Steps:**
```python
bot = Bot(token="xxx", base_url="http://localhost:9800")
bot.run()
```

**Expected Result:**
- WebSocket connection established
- Authentication successful
- Ready to receive messages
- Auto-reconnect on disconnect

---

### TC-SDK-002: Streaming Response Context Manager
**Priority:** High
**Component:** Stream Context

**Preconditions:**
- Bot connected
- Message received

**Steps:**
```python
async with ctx.stream(phase="thinking") as s:
    await s.update("Processing...", progress=0.5)
    s.result = "Complete!"
```

**Expected Result:**
- Stream started with correct phase
- Updates sent in real-time
- Stream ended with result
- No resource leaks

---

### TC-SDK-003: Task Management Integration
**Priority:** Medium
**Component:** Task API

**Preconditions:**
- Bot connected
- Conversation active

**Steps:**
```python
from agent_im_python import TaskCreate

task = await bot.api.create_task(
    conversation_id=1,
    TaskCreate(title="Test task", priority="high")
)
await bot.api.complete_task(task.id)
```

**Expected Result:**
- Task created successfully
- Task ID returned
- Status update successful
- Task marked as done

---

### TC-SDK-004: Error Handling with New Format
**Priority:** High
**Component:** Error Classes

**Preconditions:**
- Invalid request prepared

**Steps:**
```python
try:
    await bot.api.send_message(invalid_data)
except APIError as e:
    assert e.code == "VALIDATION_ERROR"
    assert e.request_id is not None
    assert e.details["field"] == "layers"
```

**Expected Result:**
- Structured error caught
- All error fields accessible
- Backward compatible
- Clear error message

---

## 6. WebSocket Transport Tests

### TC-WS-001: Connection Stability Over Time
**Priority:** High
**Component:** WebSocket Client

**Preconditions:**
- Long-running connection

**Steps:**
1. Establish WebSocket connection
2. Send ping every 30 seconds
3. Run for 24 hours
4. Monitor for disconnections
5. Verify message delivery

**Expected Result:**
- Connection stable for 24h
- Automatic reconnection if needed
- No message loss
- Memory usage stable

---

### TC-WS-002: Message Ordering Guarantee
**Priority:** High
**Component:** WebSocket Protocol

**Preconditions:**
- Active WebSocket connection

**Steps:**
1. Send 100 messages rapidly
2. Each with sequential ID
3. Verify receipt order
4. Check for gaps
5. Verify no duplicates

**Expected Result:**
- All messages received
- Order preserved (1→100)
- No gaps in sequence
- No duplicates

---

### TC-WS-003: Reconnection with Exponential Backoff
**Priority:** Medium
**Component:** WebSocket Reconnection

**Preconditions:**
- WebSocket connected

**Steps:**
1. Disconnect WebSocket
2. Observe reconnection attempts
3. Verify delays: 1s, 2s, 4s, 8s, 16s, 32s
4. Verify max delay cap (60s)
5. Successful reconnection resets

**Expected Result:**
- Exponential backoff applied
- Max delay capped
- Counter reset on success
- No aggressive reconnection

---

## 7. Performance Tests

### TC-PERF-001: Message Throughput
**Priority:** Medium
**Component:** Message Pipeline

**Preconditions:**
- Load testing environment

**Steps:**
1. Connect 100 bot clients
2. Each sends 10 msg/second
3. Run for 5 minutes
4. Measure delivery rate
5. Check for bottlenecks

**Expected Result:**
- 1000 msg/s sustained
- <100ms delivery latency (p95)
- No message loss
- CPU usage <80%
- Memory stable

---

### TC-PERF-002: Search Performance with Large Dataset
**Priority:** Medium
**Component:** Search System

**Preconditions:**
- 100k messages in database

**Steps:**
1. Search for common term
2. Search for rare term
3. Search with filters
4. Measure response times
5. Check result accuracy

**Expected Result:**
- <500ms response time
- Accurate results
- Pagination works
- No timeout errors
- Relevance ranking correct

---

## 8. Integration Tests

### TC-INT-001: End-to-End Message Flow
**Priority:** High
**Component:** Full System

**Preconditions:**
- All services running

**Steps:**
1. User sends message via UI
2. Backend receives via WebSocket
3. Bot receives via SDK
4. Bot processes and replies
5. User sees reply in UI

**Expected Result:**
- Complete flow in <2 seconds
- All components communicate
- Message integrity maintained
- Proper error handling
- UI updates correctly

---

### TC-INT-002: Multi-Bot Conversation
**Priority:** Medium
**Component:** Group Conversation

**Preconditions:**
- Multiple bots online

**Steps:**
1. Create group conversation
2. Add 3 different bots
3. User sends question
4. All bots receive message
5. Each bot responds

**Expected Result:**
- All bots receive message
- Responses don't conflict
- UI handles multiple responses
- Message order preserved
- No performance degradation

---

## 9. Security Tests

### TC-SEC-001: Token Authentication
**Priority:** High
**Component:** Auth System

**Preconditions:**
- Valid and invalid tokens

**Steps:**
1. Connect with valid token → Success
2. Connect with invalid token → Rejected
3. Connect with expired token → Rejected
4. Use revoked token → Rejected
5. Token in wrong format → Rejected

**Expected Result:**
- Only valid tokens accepted
- Clear error messages
- Failed attempts logged
- No token leakage in logs
- Rate limiting applied

---

### TC-SEC-002: SQL Injection Prevention
**Priority:** High
**Component:** Database Layer

**Preconditions:**
- Malicious input prepared

**Steps:**
1. Send message with SQL: `'; DROP TABLE messages; --`
2. Search with SQL injection attempt
3. Create task with SQL in fields
4. Update conversation with SQL
5. Check database integrity

**Expected Result:**
- All attempts safely handled
- No database corruption
- Parameterized queries used
- Input properly escaped
- Error messages safe

---

## 10. Regression Tests

### TC-REG-001: Bot Soft Delete Functionality
**Priority:** High
**Component:** Entity Management

**Preconditions:**
- Active bot entity

**Steps:**
1. "Delete" bot (soft delete)
2. Verify status = 'disabled'
3. Verify bot still in list
4. Send message to group
5. Verify disabled bot doesn't receive

**Expected Result:**
- Bot disabled, not deleted
- Still visible in UI (grayed out)
- Can be reactivated
- No messages to disabled bots
- Conversation history preserved

---

## Test Execution Matrix

| Test Category | Critical | High | Medium | Low | Total |
|--------------|----------|------|--------|-----|-------|
| Message Deduplication | 2 | 1 | 0 | 0 | 3 |
| Accessibility | 3 | 1 | 0 | 0 | 4 |
| Error Handling | 2 | 1 | 0 | 0 | 3 |
| Task Management | 2 | 2 | 0 | 0 | 4 |
| Python SDK | 3 | 1 | 0 | 0 | 4 |
| WebSocket | 2 | 1 | 0 | 0 | 3 |
| Performance | 0 | 0 | 2 | 0 | 2 |
| Integration | 1 | 1 | 0 | 0 | 2 |
| Security | 2 | 0 | 0 | 0 | 2 |
| Regression | 1 | 0 | 0 | 0 | 1 |
| **Total** | **18** | **8** | **2** | **0** | **28** |

## Automation Priority

### Phase 1 - Critical Path (Week 1)
- WebSocket connection tests
- Message deduplication
- Basic error handling
- Authentication flow

### Phase 2 - Core Features (Week 2)
- Task CRUD operations
- SDK integration tests
- Accessibility keyboard nav
- Search functionality

### Phase 3 - Edge Cases (Week 3)
- Performance benchmarks
- Security penetration
- Multi-bot scenarios
- Failure recovery

## Test Environment Requirements

### Infrastructure
- Backend: Go 1.21+, PostgreSQL 15+
- Frontend: Node 18+, React 18
- Python SDK: Python 3.10+
- Load Testing: K6 or JMeter
- Browser Testing: Chrome, Firefox, Safari

### Test Data
- 10 test user accounts
- 5 test bot accounts
- 1000 seed messages
- 100 test conversations
- 50 test tasks

### Monitoring
- Application logs
- Performance metrics
- Error tracking (Sentry)
- Database query logs
- Network traffic capture

## Success Criteria

- All critical tests passing
- >95% high priority tests passing
- <2% test flakiness
- <5 second test execution time (unit)
- <30 minute full regression suite
- Zero security vulnerabilities
- WCAG AA compliance verified
