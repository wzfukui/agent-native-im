# Agent-Native IM Platform User Stories

## Version 4.2 - Stability, Session Recovery, and Delivery Reliability

This document contains comprehensive user stories covering all platform capabilities, organized by user type and feature area.

> **Changelog:** v4.2 (2026-03-27) adds acceptance criteria for cookie-restored invite access, stable avatar delivery, safer build refresh detection, reduced context-card refetching, and hardened token rotation behavior.

---

## 1. Platform Users

### 1.1 Developer (Bot Creator)

#### Story: Creating and Deploying a Bot
**As a** developer
**I want to** create and deploy an AI bot on the platform
**So that** users can interact with my AI service

**Acceptance Criteria:**
- [ ] Can create a new bot entity via API or SDK
- [ ] Can obtain authentication token for the bot
- [ ] Can implement message handlers in Python/JS/Go SDK
- [ ] Can deploy bot to respond to user messages
- [ ] Can monitor bot status (online/offline)
- [ ] Can update bot configuration and metadata

**Example Implementation:**
```python
bot = Bot(token="xxx", base_url="https://api.agent-im.com")

@bot.on_message
async def handle(ctx, msg):
    response = await process_user_query(msg.layers.summary)
    await ctx.reply(summary=response)

bot.run()
```

#### Story: Implementing Streaming Responses
**As a** developer
**I want to** show real-time progress to users
**So that** they understand what my bot is doing

**Acceptance Criteria:**
- [ ] Can start a streaming session with phase indicator
- [ ] Can send incremental updates with progress
- [ ] Can show "thinking" visualization to users
- [ ] Can finalize stream with persistent result
- [ ] Users see smooth, real-time updates

#### Story: Managing Tasks Within Conversations
**As a** developer
**I want to** create and track tasks in conversations
**So that** work items are organized and trackable

**Acceptance Criteria:**
- [ ] Can create tasks with title, description, priority
- [ ] Can assign tasks to users or bots
- [ ] Can update task status (pending/in_progress/done)
- [ ] Can set due dates and dependencies
- [ ] Can query tasks by conversation or status

#### Story: Debugging Bot Integration
**As a** developer
**I want to** enable debug mode and trace request flows
**So that** I can quickly diagnose issues with my bot

**Acceptance Criteria:**
- [ ] Can enable debug logging via `Bot(token="xxx", debug=True)`
- [ ] Can enable debug at runtime via `Bot.enable_debug()`
- [ ] All SDK modules (api, ws, bot, context) emit DEBUG-level messages
- [ ] API requests include trace IDs via `X-Request-ID` header
- [ ] API responses log status code, timing (elapsed ms), and server request ID
- [ ] WebSocket frame send/receive events are logged
- [ ] Memory cache hit/miss is logged for context loading
- [ ] Context operations (reply, stream, handover) are logged with conversation ID

**Example Implementation:**
```python
# Enable at construction
bot = Bot(token="xxx", base_url="http://localhost:9800", debug=True)

# Or enable at runtime
Bot.enable_debug()

# Debug output shows trace IDs and timing:
# 14:32:01 [agent_im.api] DEBUG api: POST /api/v1/messages/send → 200 (42ms) trace=a1b2c3d4e5f6 req=req_0f3a9c_182345
```

#### Story: Using Conversation Memory
**As a** developer
**I want to** store and retrieve key-value memories per conversation
**So that** my bot maintains context across messages

**Acceptance Criteria:**
- [ ] Can store a memory via `ctx.remember(key, content)` (upsert by key)
- [ ] Can recall all memories via `ctx.recall()` or a specific one via `ctx.recall(key)`
- [ ] Can delete a memory via `ctx.forget(memory_id)`
- [ ] Memories are auto-loaded into `ctx.memories` dict on each incoming message
- [ ] Can build LLM system context from memories via `ctx.get_system_context()`
- [ ] Memory cache is invalidated when `conversation.memory_updated` event is received
- [ ] Memory changes are broadcast to all conversation participants via WebSocket

**Example Implementation:**
```python
@bot.on_message
async def handle(ctx, msg):
    # Memories are pre-loaded into ctx.memories
    user_prefs = ctx.memories.get("user_preferences", "")

    # Store new memory
    await ctx.remember("last_topic", "quarterly report")

    # Build system prompt with all memories
    system = ctx.get_system_context() + "\n\nYou are a helpful assistant."

    # Recall a specific memory later
    topic = await ctx.recall("last_topic")

    # Delete a memory
    await ctx.forget(memory_id=42)
```

#### Story: Agent Collaboration via Handover
**As a** developer
**I want to** perform structured task handovers between agents
**So that** agents can collaborate on multi-step workflows

**Acceptance Criteria:**
- [ ] Can send a handover via `ctx.handover(assign_to, summary, ...)`
- [ ] Handover supports types: `task_completion`, `bug_report`, `review_request`, `status_report`
- [ ] Handover message includes deliverables, task references, and context
- [ ] Handover messages are sent with `content_type="task_handover"` and mention assigned agents
- [ ] Receiving bot can register `@bot.on_handover` to handle handover messages
- [ ] Handover data (type, deliverables, context) is parsed and passed to handler
- [ ] Regular `@bot.on_message` is skipped for handover messages when a dedicated handler exists

**Example Implementation:**
```python
# Sending agent: hand over completed work
await ctx.handover(
    assign_to=[reviewer_bot_id],
    summary="Code review needed for auth module",
    handover_type="review_request",
    deliverables=[{"type": "code_diff", "url": "/files/diff_abc.patch"}],
    context={"branch": "feature/auth", "priority": "high"},
)

# Receiving agent: handle incoming handover
@bot.on_handover
async def handle_handover(ctx, msg, handover_data):
    htype = handover_data.get("handover_type")
    deliverables = handover_data.get("deliverables", [])
    await ctx.reply(summary=f"Received {htype}, processing {len(deliverables)} deliverables...")
```

#### Story: Structured Mentions
**As a** developer
**I want to** @mention other entities with structured intent
**So that** mentions carry actionable meaning beyond a plain notification

**Acceptance Criteria:**
- [ ] Can send a structured mention via `ctx.mention(entity_ids, summary, intent_type, ...)`
- [ ] Supported intent types: `task_assign`, `question`, `review`, `fyi`
- [ ] Mention payload includes instruction, priority, and optional context references
- [ ] Mention data is embedded in `layers.data.mention_intent`
- [ ] Mentioned entities receive the message with mention notification
- [ ] Priority levels supported: `low`, `medium`, `high`, `urgent`

**Example Implementation:**
```python
await ctx.mention(
    entity_ids=[agent_42],
    summary="Please review the deployment plan",
    intent_type="review",
    instruction="Check the rollback strategy section",
    priority="high",
    context_refs=[{"type": "message", "id": 1234}],
)
```

#### Story: Entity Discovery
**As a** developer
**I want to** search for agents by capability
**So that** my bot can find and collaborate with the right agents

**Acceptance Criteria:**
- [ ] Can search entities via `GET /entities/search?capability=...`
- [ ] Search matches against entity capabilities (max 100 chars query)
- [ ] Results include entity details and online/offline status
- [ ] Only active entities are returned
- [ ] Can be used to dynamically discover agents for handover or mention

---

### 1.2 End User (Human)

#### Story: Starting a Conversation with a Bot
**As a** user
**I want to** start a conversation with an AI bot
**So that** I can get help with my tasks

**Acceptance Criteria:**
- [ ] Can see list of available bots
- [ ] Can view bot descriptions and capabilities
- [ ] Can initiate conversation with chosen bot
- [ ] Conversation is created with unique ID
- [ ] Can send messages and receive responses
- [ ] Conversation history is preserved

#### Story: Managing Multiple Conversations
**As a** user
**I want to** manage multiple bot conversations
**So that** I can organize different topics and tasks

**Acceptance Criteria:**
- [ ] Can switch between active conversations
- [ ] Can see conversation list with titles
- [ ] Can rename conversations for clarity
- [ ] Can search through conversation history
- [ ] Can delete old conversations
- [ ] Unread message indicators are shown

#### Story: Collaborating in Group Conversations
**As a** user
**I want to** collaborate with others through bots
**So that** we can work together effectively

**Acceptance Criteria:**
- [ ] Can create group conversations
- [ ] Can invite other users to join
- [ ] Can add multiple bots to a group
- [ ] All participants see messages in real-time
- [ ] Can mention specific participants
- [ ] Can see who is currently online

#### Story: Tracking Work with Tasks
**As a** user
**I want to** track tasks within conversations
**So that** I can manage my workload effectively

**Acceptance Criteria:**
- [ ] Can view all tasks in a conversation
- [ ] Can see task status and priority
- [ ] Can mark tasks as complete
- [ ] Can see overdue tasks highlighted
- [ ] Can filter tasks by status/assignee
- [ ] Receive notifications for task updates

#### Story: Reacting to Messages
**As a** user
**I want to** add emoji reactions to messages
**So that** I can express quick feedback without sending a new message

**Acceptance Criteria:**
- [ ] Can add a reaction to any message in a conversation I participate in
- [ ] Clicking the same reaction again removes it (toggle behavior)
- [ ] Can see all reactions on a message with counts
- [ ] Reactions are broadcast to all participants in real-time via WebSocket
- [ ] Emoji length is validated (max 32 characters)
- [ ] Both users and bots can add reactions

#### Story: Editing Sent Messages
**As a** user
**I want to** edit a message I already sent
**So that** I can correct mistakes or update information

**Acceptance Criteria:**
- [ ] Can edit message content via `PUT /messages/:id`
- [ ] Edit updates the message layers (summary, data)
- [ ] Edit is reflected for all conversation participants
- [ ] Only the original sender can edit their message
- [ ] Bots can edit their own messages via `ctx.edit_message(message_id, summary, data)`

#### Story: Uploading and Sharing Files
**As a** user
**I want to** upload and share files in conversations
**So that** I can collaborate on documents, images, and other media

**Acceptance Criteria:**
- [ ] Can upload files up to 32 MB via `POST /files/upload`
- [ ] Allowed file types: images, audio, video, text, PDF, Office documents, archives
- [ ] Uploaded files return a URL, filename, and size
- [ ] File names are sanitized to avoid encoding issues (safe format: `YYYYMMDD_HHMMSS_hex.ext`)
- [ ] Files are served via static file path at `/files/`
- [ ] Bots can upload and send files via `ctx.upload_file(path)` and `ctx.send_file(path, summary)`

#### Story: Viewing Bot Integration Status
**As a** user
**I want to** check the health and connectivity of my bots
**So that** I can diagnose and fix integration issues

**Acceptance Criteria:**
- [ ] Can run self-check via `GET /entities/:id/self-check`
- [ ] Self-check reports: entity status, online/offline, credential status (bootstrap/API key), and readiness
- [ ] Self-check provides actionable recommendations (e.g., "agent is offline, verify network")
- [ ] Can view detailed diagnostics via `GET /entities/:id/diagnostics`
- [ ] Diagnostics include: connection count, disconnect history, connected devices, last seen time
- [ ] Diagnostics include hub-level stats (total WebSocket connections)

---

### 1.3 Administrator

#### Story: Managing Platform Bots
**As an** administrator
**I want to** manage all bots on the platform
**So that** I can ensure quality and security

**Acceptance Criteria:**
- [ ] Can view all registered bots
- [ ] Can disable/enable bots
- [ ] Can view bot usage statistics
- [ ] Can revoke bot tokens if needed
- [ ] Can set rate limits per bot
- [ ] Can audit bot message logs

#### Story: Monitoring Platform Health
**As an** administrator
**I want to** monitor platform health and performance
**So that** I can ensure reliable service

**Acceptance Criteria:**
- [ ] Can view real-time connection metrics
- [ ] Can see message throughput statistics
- [ ] Can identify performance bottlenecks
- [ ] Can view error rates and types
- [ ] Can access detailed audit logs
- [ ] Receive alerts for critical issues

---

## 2. Feature-Based Stories

### 2.1 Authentication & Security

#### Story: Secure Bot Authentication
**As a** bot developer
**I want to** authenticate my bot securely
**So that** only authorized bots can access the platform

**Acceptance Criteria:**
- [ ] Bot tokens are cryptographically secure
- [ ] Tokens can be revoked and regenerated
- [ ] Failed auth attempts are logged
- [ ] Rate limiting prevents brute force
- [ ] Tokens expire after configurable period

#### Story: User Login with GitHub
**As a** user
**I want to** login with my GitHub account
**So that** I don't need another password

**Acceptance Criteria:**
- [ ] Can click "Login with GitHub"
- [ ] Redirected to GitHub for authorization
- [ ] Profile automatically populated from GitHub
- [ ] Can link multiple GitHub accounts
- [ ] Can revoke GitHub access

---

### 2.2 Real-time Communication

#### Story: WebSocket Connection Management
**As a** bot developer
**I want** reliable WebSocket connections
**So that** my bot stays connected 24/7

**Acceptance Criteria:**
- [ ] Auto-reconnect on connection loss
- [ ] Exponential backoff for retries
- [ ] Message queue during disconnection
- [ ] No duplicate messages after reconnect
- [ ] Connection status is monitored

#### Story: Message Deduplication
**As a** platform developer
**I want to** prevent duplicate messages
**So that** users have a clean experience

**Acceptance Criteria:**
- [ ] Messages have unique IDs
- [ ] Duplicates detected and filtered
- [ ] Order preserved within conversations
- [ ] Works across reconnections
- [ ] Handles out-of-order delivery

---

### 2.3 UI/UX & Accessibility

#### Story: Keyboard Navigation
**As a** power user
**I want to** navigate with keyboard shortcuts
**So that** I can work efficiently

**Acceptance Criteria:**
- [ ] Cmd/Ctrl+K opens search
- [ ] Tab navigates between elements
- [ ] Enter sends messages
- [ ] Escape closes modals
- [ ] Shortcuts are customizable
- [ ] Help menu shows all shortcuts

#### Story: Screen Reader Support
**As a** visually impaired user
**I want** full screen reader support
**So that** I can use the platform independently

**Acceptance Criteria:**
- [ ] All elements have ARIA labels
- [ ] Focus order is logical
- [ ] Status updates are announced
- [ ] Keyboard navigation works throughout
- [ ] Color contrast meets WCAG AA
- [ ] Images have alt text

#### Story: Mobile Responsive Design
**As a** mobile user
**I want** a responsive interface
**So that** I can use the platform on any device

**Acceptance Criteria:**
- [ ] Layout adapts to screen size
- [ ] Touch targets are appropriately sized
- [ ] Swipe gestures work naturally
- [ ] Virtual keyboard doesn't obscure UI
- [ ] Performance is optimized for mobile

---

### 2.4 Bot Capabilities

#### Story: Interactive Messages
**As a** bot developer
**I want to** send interactive messages
**So that** users can make choices easily

**Acceptance Criteria:**
- [ ] Can send choice buttons
- [ ] Can send confirmation dialogs
- [ ] Can send forms for data collection
- [ ] User selections are captured
- [ ] Can update message after interaction
- [ ] Supports keyboard selection

#### Story: File and Media Handling
**As a** bot developer
**I want to** handle files and media
**So that** I can process documents and images

**Acceptance Criteria:**
- [ ] Can receive uploaded files
- [ ] Can send files to users
- [ ] Supports common formats (PDF, images, etc.)
- [ ] File size limits are enforced
- [ ] Files are virus scanned
- [ ] Preview generation for images

#### Story: Multi-language Support
**As a** bot developer
**I want to** support multiple languages
**So that** I can serve global users

**Acceptance Criteria:**
- [ ] Can detect user language preference
- [ ] Can send localized responses
- [ ] UI supports language switching
- [ ] Date/time formats are localized
- [ ] RTL languages are supported

---

### 2.5 Task Management

#### Story: Creating Subtasks
**As a** user
**I want to** break tasks into subtasks
**So that** I can manage complex projects

**Acceptance Criteria:**
- [ ] Can create subtasks under parent task
- [ ] Parent task shows subtask count
- [ ] Subtasks inherit conversation context
- [ ] Completing subtasks updates parent progress
- [ ] Can convert task to subtask
- [ ] Maximum nesting depth enforced

#### Story: Task Dependencies
**As a** project manager
**I want to** set task dependencies
**So that** work flows in correct order

**Acceptance Criteria:**
- [ ] Can set "blocked by" relationships
- [ ] Blocked tasks show warning
- [ ] Can't start blocked tasks
- [ ] Completing blocker unblocks tasks
- [ ] Dependency graph visualization
- [ ] Circular dependencies prevented

#### Story: Task Notifications
**As a** user
**I want** notifications for task changes
**So that** I stay informed

**Acceptance Criteria:**
- [ ] Notified when assigned a task
- [ ] Notified of task status changes
- [ ] Notified of approaching due dates
- [ ] Can configure notification preferences
- [ ] In-app and email notifications
- [ ] Can mute specific tasks

---

### 2.6 Error Handling

#### Story: Graceful Error Recovery
**As a** user
**I want** clear error messages
**So that** I understand what went wrong

**Acceptance Criteria:**
- [ ] Errors show human-readable messages
- [ ] Suggest corrective actions
- [ ] Include request ID for support
- [ ] Don't expose sensitive information
- [ ] Retry options where appropriate
- [ ] Errors are logged for debugging

#### Story: Offline Mode
**As a** user
**I want** basic offline functionality
**So that** I can work without internet

**Acceptance Criteria:**
- [ ] Can read cached conversations
- [ ] Can draft messages offline
- [ ] Clear offline indicator shown
- [ ] Queued actions sync when online
- [ ] Conflict resolution for edits
- [ ] No data loss on reconnection

---

## 3. Performance & Scalability Stories

#### Story: Message Delivery Performance
**As a** user
**I want** instant message delivery
**So that** conversations feel natural

**Acceptance Criteria:**
- [ ] Messages deliver in <100ms (same region)
- [ ] Typing indicators update in real-time
- [ ] Read receipts update immediately
- [ ] No lag with 100+ participants
- [ ] Smooth scrolling with 10k+ messages

#### Story: Search Performance
**As a** user
**I want** fast message search
**So that** I can find information quickly

**Acceptance Criteria:**
- [ ] Search returns results in <500ms
- [ ] Supports full-text search
- [ ] Can filter by date/sender/conversation
- [ ] Highlights matching terms
- [ ] Pagination for large result sets
- [ ] Search suggestions as-you-type

---

## 4. DevOps & Deployment Stories

#### Story: Zero-Downtime Deployment
**As a** platform operator
**I want** zero-downtime deployments
**So that** users aren't disrupted

**Acceptance Criteria:**
- [ ] Rolling updates for backend
- [ ] WebSocket connections migrate gracefully
- [ ] Database migrations are backward compatible
- [ ] Frontend shows update notification
- [ ] Can rollback if issues detected
- [ ] Health checks before traffic routing

#### Story: Auto-scaling
**As a** platform operator
**I want** automatic scaling
**So that** we handle traffic spikes

**Acceptance Criteria:**
- [ ] Scales based on connection count
- [ ] Scales based on message throughput
- [ ] Maintains session affinity
- [ ] Distributes load evenly
- [ ] Scales down during quiet periods
- [ ] Cost optimization rules applied

---

## 5. Integration Stories

#### Story: Webhook Integration
**As a** developer
**I want** webhook notifications
**So that** I can integrate with external systems

**Acceptance Criteria:**
- [ ] Can register webhook endpoints
- [ ] Configurable event types
- [ ] Retry logic for failed deliveries
- [ ] Webhook signatures for security
- [ ] Delivery logs available
- [ ] Rate limiting per endpoint

#### Story: API Integration
**As a** developer
**I want** comprehensive REST APIs
**So that** I can build custom integrations

**Acceptance Criteria:**
- [ ] Full CRUD for all resources
- [ ] OpenAPI specification provided
- [ ] Consistent error responses
- [ ] Pagination for list endpoints
- [ ] Filtering and sorting options
- [ ] Rate limiting with clear headers

---

## Test Scenarios

### Scenario 1: New User Onboarding
1. User visits platform homepage
2. Clicks "Sign Up" or "Login with GitHub"
3. Completes authentication flow
4. Sees welcome screen with bot list
5. Starts first conversation with a bot
6. Sends message and receives response
7. Explores UI features and settings

### Scenario 2: Bot Development Lifecycle
1. Developer reads API documentation
2. Creates bot entity via API
3. Implements bot logic using SDK
4. Tests bot in development environment
5. Deploys bot to production
6. Monitors bot performance and logs
7. Updates bot based on user feedback

### Scenario 3: Team Collaboration
1. User creates group conversation
2. Invites team members to join
3. Adds relevant bots to group
4. Team discusses project requirements
5. Bot creates tasks from discussion
6. Team members claim and complete tasks
7. Progress tracked through task dashboard

### Scenario 4: Error Recovery
1. User loses internet connection
2. Platform shows offline indicator
3. User continues reading cached content
4. Connection restored
5. Queued messages are sent
6. User receives missed messages
7. Conversation continues seamlessly

### Scenario 5: Agent-to-Agent Collaboration
1. User asks Bot A to complete a complex task
2. Bot A determines it needs Bot B's expertise
3. Bot A searches for agents with the required capability via `/entities/search`
4. Bot A sends a structured handover to Bot B with deliverables and context
5. Bot B receives the handover via `@bot.on_handover` handler
6. Bot B processes the deliverables and stores progress in conversation memory
7. Bot B sends a handover back to Bot A with completed results
8. Bot A summarizes the collaborative outcome to the user

### Scenario 6: Debug and Troubleshoot Bot
1. Developer notices bot is not responding to messages
2. Developer enables debug mode: `Bot(token="xxx", debug=True)`
3. Debug logs show WebSocket connection status and frame traffic
4. Developer checks self-check endpoint: `GET /entities/:id/self-check`
5. Self-check reveals bot is offline with recommendation to verify network
6. Developer fixes network issue and reconnects
7. Debug logs confirm successful WebSocket handshake and message dispatch
8. Developer checks diagnostics for connection stability (disconnect counts, devices)
9. API trace IDs (`X-Request-ID`) correlate SDK logs with server-side logs

---

## Success Metrics

### User Metrics
- Daily Active Users (DAU)
- Messages sent per day
- Average session duration
- User retention rate (7-day, 30-day)
- Time to first message
- Conversation completion rate

### Bot Metrics
- Bot response time (p50, p95, p99)
- Bot availability/uptime
- Messages handled per bot
- Error rate per bot
- User satisfaction rating
- Task completion rate

### Platform Metrics
- System uptime (99.9% target)
- API response time
- WebSocket connection stability
- Message delivery success rate
- Search query performance
- Database query performance

### Business Metrics
- New user sign-ups
- Bot developer acquisitions
- Premium subscription conversions
- Support ticket volume
- Feature adoption rate
- Platform growth rate

---

## 7. v4.0 Agent Onboarding Stories

### 7.1 Transparent Key Lifecycle

#### Story: Seamless Bootstrap-to-Permanent Key Upgrade
**As a** bot developer
**I want** the SDK to automatically handle key upgrades
**So that** I don't need to manually manage credential lifecycle

**Acceptance Criteria:**
- [x] SDK accepts `key_file` parameter (default `.agent_im_key`)
- [x] On `connection.approved` WS event, SDK extracts `api_key`
- [x] SDK auto-updates API client headers and WS reconnect URL
- [x] Permanent key is persisted to key_file
- [x] On restart, SDK loads saved permanent key
- [x] Developer never needs to see or manage bootstrap vs permanent keys

### 7.2 Orphan Stream Recovery

#### Story: Stale Stream Auto-Cleanup
**As a** user viewing a conversation
**I want** stale "processing..." indicators to disappear automatically
**So that** crashed bot streams don't block the UI forever

**Acceptance Criteria:**
- [x] Frontend runs 15-second interval checking all active streams
- [x] Streams older than 2 minutes with no updates are marked as timed out
- [x] Timed-out streams display "timed out" instead of infinite spinner
- [x] Cleanup interval stops when component unmounts (no memory leak)

### 7.3 Group Subscription Filtering

#### Story: Smart Message Filtering in Group Conversations
**As a** bot in a group conversation
**I want** to only receive messages I'm supposed to handle
**So that** I don't waste resources processing irrelevant messages

**Acceptance Criteria:**
- [x] `entity.config` WS event populates subscription_config map
- [x] `mention_only` mode: bot skips messages where it's not mentioned
- [x] `subscribe_all` mode: bot receives all messages (default)
- [x] `Message.is_mentioned(entity_id)` helper checks `mentioned_entity_ids`
- [x] Bot-level filter is opt-out via `filter_by_subscription=False`

### 7.4 File Download Capability

#### Story: Downloading Attachments from Messages
**As a** bot receiving file messages
**I want** to download and process attachment content
**So that** I can analyze documents, images, or data files

**Acceptance Criteria:**
- [x] `ctx.download_attachment(attachment, dest_dir)` saves file locally
- [x] `ctx.get_attachment_content(attachment)` returns bytes in memory
- [x] Relative `/files/` URLs are resolved against base_url automatically
- [x] Downloads use existing auth headers

### 7.5 LLM Integration Tutorial

#### Story: From Echo Bot to AI Agent
**As a** new bot developer
**I want** a working LLM integration example in the quickstart
**So that** I can build a real AI bot, not just an echo bot

**Acceptance Criteria:**
- [x] `examples/llm_quickstart.py` provides ~30-line OpenAI integration
- [x] Bot quickstart Step 2 shows real LLM code with streaming
- [x] Example uses `ctx.get_system_context()` for memory injection
- [x] Ghost `@bot.on_health_check` API removed from documentation

### 7.6 Email Login & Settings

#### Story: Login with Username or Email
**As a** user
**I want** to log in with either my username or email
**So that** I have a familiar, flexible authentication experience

**Acceptance Criteria:**
- [x] Login form accepts username or email in the same field
- [x] Backend detects `@` in input → tries email lookup first, falls back to username
- [x] Registration form includes optional email field
- [x] Settings page includes email field for profile editing
- [x] Email column has partial unique index (non-empty emails must be unique)

---

## 8. v4.1 UX & Agent Context Stories (2026-03-13)

### 8.1 Invite Link Flow

#### Story: Joining a Conversation via Invite Link
**As a** user who received an invite link
**I want to** click the link, see the conversation info, and join with one tap
**So that** I can quickly start collaborating without manual setup

**Acceptance Criteria:**
- [x] `/join/:code` URL is detected on app load and routes to the Join page
- [x] Join page displays conversation title, description, and participant count via `GET /api/v1/invites/:code`
- [x] "Join" button calls `POST /api/v1/invites/:code/join` and adds user as participant
- [x] After joining, user is navigated directly into the conversation (not the conversation list)
- [x] Expired, invalid, or already-joined states are handled with clear error messages
- [x] Unauthenticated users are redirected to login, then back to the invite page

**Test Cases:**
| # | Test Case | Steps | Expected Result |
|---|-----------|-------|-----------------|
| 1 | Valid invite link | Open `/join/abc123` while logged in | Shows conversation info + Join button |
| 2 | Join successfully | Click Join button | Navigates to the conversation, conversation appears in sidebar |
| 3 | Expired invite | Open `/join/expired_code` | Shows "invite expired" error |
| 4 | Already joined | Open invite for a conversation user is already in | Shows "already a member" message |
| 5 | Not logged in | Open `/join/abc123` while not authenticated | Redirects to login, then back to invite page |
| 6 | Double-click prevention | Rapidly click Join twice | Only one API call made, no duplicate participant entries |

### 8.2 Conversation Persistence Across Refresh

#### Story: Preserving Active Conversation on Page Refresh
**As a** user in the middle of a conversation
**I want to** refresh the page and return to the same conversation
**So that** I don't lose my place and have to navigate back manually

**Acceptance Criteria:**
- [x] Active conversation ID is persisted to URL hash (`#c=<id>`) on selection
- [x] On page load, hash is parsed and active conversation is restored
- [x] Hash is cleared when user navigates away from all conversations
- [x] Browser back/forward does not create extra history entries (uses `replaceState`)

**Test Cases:**
| # | Test Case | Steps | Expected Result |
|---|-----------|-------|-----------------|
| 1 | Refresh preserves conversation | Open conversation → Refresh page | Same conversation is active after reload |
| 2 | Hash updates on switch | Switch between conversations | URL hash changes to match active conversation |
| 3 | No hash when no active conv | Close/deselect conversation | URL hash is cleared |
| 4 | Invalid hash ignored | Set `#c=99999999` (non-existent) | App loads normally, shows conversation list |

### 8.3 Bot Thinking Animation

#### Story: Showing Bot "Thinking" Indicator While Waiting for Response
**As a** user who just sent a message to a bot
**I want to** see a visual indicator that the bot is processing my message
**So that** I know the system is working and my message was received

**Acceptance Criteria:**
- [x] After sending a message to a bot (DM or @mention in group), a pulsing dot animation appears
- [x] Animation shows the bot's avatar and name above the dots
- [x] Animation disappears when: bot replies, bot starts streaming, bot sends typing indicator, or 60s timeout
- [x] Animation does not appear for human-to-human messages
- [x] Animation is hidden when streaming bubbles are active (no duplicate indicators)

**Test Cases:**
| # | Test Case | Steps | Expected Result |
|---|-----------|-------|-----------------|
| 1 | DM thinking indicator | Send message in DM with bot | 3-dot pulsing animation appears below sent message |
| 2 | Group @mention thinking | Send `@bot question` in group | Thinking animation appears for the mentioned bot |
| 3 | Cleared on bot reply | Wait for bot response | Animation disappears when bot message arrives |
| 4 | Cleared on stream start | Bot begins streaming response | Animation disappears, streaming bubble takes over |
| 5 | 60s timeout | Bot doesn't respond within 60s | Animation auto-hides after timeout |
| 6 | Conversation switch | Switch to another conversation while thinking | Animation is cleared for the previous conversation |

### 8.4 Private Message Archive

#### Story: Archiving Direct Messages
**As a** user with many DM conversations
**I want to** archive old DMs just like I can archive group conversations
**So that** I can keep my conversation list clean and focused

**Acceptance Criteria:**
- [x] Archive option appears in the context menu for DM conversations (not just groups)
- [x] Archived DMs behave the same as archived groups (hidden from main list, viewable in archive)

**Test Cases:**
| # | Test Case | Steps | Expected Result |
|---|-----------|-------|-----------------|
| 1 | Archive DM | Right-click DM → Archive | DM moves to archive section |
| 2 | Unarchive DM | Open archived DMs → Unarchive | DM returns to main list |

### 8.5 Text Message Attachments Display

#### Story: Displaying File Attachments on Text Messages
**As a** user viewing a conversation
**I want to** see file attachments rendered inline with text messages
**So that** I can view images and download files without extra clicks

**Acceptance Criteria:**
- [x] Text messages with image attachments show thumbnail grid below the text
- [x] Clicking an image attachment opens the lightbox viewer
- [x] Non-image file attachments show as download links with filename, type, and size
- [x] Multiple attachments of mixed types are rendered correctly together

**Test Cases:**
| # | Test Case | Steps | Expected Result |
|---|-----------|-------|-----------------|
| 1 | Text + image attachment | Send text with an image file | Text appears above, image thumbnail below |
| 2 | Text + document attachment | Send text with a .pdf file | Text appears above, download link below |
| 3 | Multiple mixed attachments | Send text with 2 images + 1 doc | Image grid + doc download link shown |
| 4 | Image lightbox | Click on attached image thumbnail | Fullscreen lightbox opens |

### 8.6 Immediate File Upload on Selection

#### Story: Uploading Files Immediately When Selected
**As a** user composing a message with file attachments
**I want to** have files upload as soon as I select them
**So that** sending is instant when I finish typing my message

**Acceptance Criteria:**
- [x] Files begin uploading immediately upon selection (click, paste, or drag-and-drop)
- [x] Upload progress shown per file (spinning indicator during upload, checkmark on success, error on failure)
- [x] Send button is disabled while uploads are in progress (shows spinner)
- [x] Failed uploads are indicated visually; user can remove and re-attach
- [x] On send, pre-uploaded file URLs are included in the message — no additional upload wait
- [x] Optimistic message includes file attachments immediately (visible to sender right away)

**Test Cases:**
| # | Test Case | Steps | Expected Result |
|---|-----------|-------|-----------------|
| 1 | Immediate upload on select | Click attach → Select file | Spinner shown immediately, file uploads in background |
| 2 | Paste image uploads | Paste image from clipboard | Image appears in file badges with upload spinner |
| 3 | Drag-drop uploads | Drop file onto chat area | File starts uploading, message sent with attachment |
| 4 | Send blocked during upload | Select large file → Immediately click Send | Send button shows spinner, blocks until upload completes |
| 5 | Upload failure handling | Select file when server is down | File badge shows error state, can be removed |
| 6 | Optimistic render with attachments | Send text + uploaded file | Message appears immediately with file attachment visible |
| 7 | Remove pending file | Click X on uploading file badge | Upload is cancelled/ignored, file removed from list |

### 8.7 Send Status Visual Feedback

#### Story: Subtle Send Status Indication
**As a** user sending messages
**I want to** see a subtle visual cue that my message is being sent
**So that** I have confidence the system received it without visual clutter

**Acceptance Criteria:**
- [x] Messages in "sending" state render at 60% opacity
- [x] Opacity transitions to 100% when the server confirms delivery
- [x] No explicit "sending"/"sent" text labels — the opacity transition itself serves as confirmation
- [x] "Queued" and "failed" states still show explicit status badges

**Test Cases:**
| # | Test Case | Steps | Expected Result |
|---|-----------|-------|-----------------|
| 1 | Sending opacity | Send a message | Message briefly appears at 60% opacity |
| 2 | Sent transition | Message confirmed by server | Opacity smoothly transitions to 100% |
| 3 | Failed badge | Send when offline → goes to failed | Red "failed" badge shown |

### 8.8 Agent Conversation Context Injection (OpenClaw Plugin)

#### Story: Providing Full Conversation Context to AI Agent
**As a** bot developer using OpenClaw to power my ANI bot
**I want to** have the bot automatically receive conversation metadata (title, instructions, participants, memories)
**So that** the bot understands its role, the conversation purpose, and can give contextually relevant answers

**Acceptance Criteria:**
- [x] Bot's system prompt includes its own identity (`You are <bot_name>`)
- [x] Conversation instructions (prompt field) are injected as `## Instructions`
- [x] Conversation description is included
- [x] Participant list with roles and entity types is included
- [x] Conversation memories (key-value pairs) are injected as `## Conversation Memory`
- [x] Context is cached per conversation (5-minute TTL) to avoid excessive API calls
- [x] Enriched WS format (`mention_with_context`) is properly unwrapped

**Test Cases:**
| # | Test Case | Steps | Expected Result |
|---|-----------|-------|-----------------|
| 1 | Bot knows its name | Ask bot "what is your name?" | Bot responds with its display name |
| 2 | Bot follows instructions | Set conversation prompt to "Always respond in Japanese" | Bot replies in Japanese |
| 3 | Bot knows participants | Ask bot "who is in this conversation?" | Bot lists participants by name and role |
| 4 | Bot recalls memories | Add memory `project=Alpha` to conversation | Bot references "Alpha" when asked about the project |
| 5 | Cache refresh | Change conversation title, wait 5+ minutes, ask bot | Bot uses updated title |
| 6 | Group @mention with context | @mention bot in group with prior messages | Bot receives context messages and conversation metadata |

### 8.9 Agent Attachment Processing (OpenClaw Plugin)

#### Story: Bot Receiving and Reading File Attachments
**As a** user sending a file to an AI bot
**I want to** have the bot read and understand the file contents
**So that** I can discuss documents, code, and other files with the bot

**Acceptance Criteria:**
- [ ] Text-based attachments (markdown, txt, json, csv, code files ≤100KB) are downloaded and inlined into the message body
- [ ] Non-text attachments (images, PDFs, large files) are described with filename, type, and size
- [ ] File download uses the bot's API key for authentication
- [ ] Download failures are handled gracefully (fallback to description-only)
- [ ] Both DM and group message attachments are processed

**Test Cases:**
| # | Test Case | Steps | Expected Result |
|---|-----------|-------|-----------------|
| 1 | Send markdown file | Send .md file to bot | Bot reads and discusses the file content |
| 2 | Send code file | Send .py file to bot | Bot sees code and can review/discuss it |
| 3 | Send large file | Send 200KB text file | Bot receives description only (exceeds 100KB limit) |
| 4 | Send image | Send .png to bot | Bot receives `[Attachment: image.png (image/png, 45.2 KB)]` |
| 5 | File download failure | Send file, but URL is inaccessible | Bot receives `[Attachment: file.txt — download failed]` |
| 6 | Multiple attachments | Send text + 2 files | Bot receives text with both file contents/descriptions appended |

### 8.10 Voice Message Optimistic Rendering

#### Story: Instant Voice Message Display
**As a** user recording a voice message
**I want to** see it appear in the chat immediately after I stop recording
**So that** the interaction feels responsive even while the file uploads

**Acceptance Criteria:**
- [x] Voice message appears in chat immediately with a temporary ID and "sending" opacity
- [x] File upload and message send happen in the background
- [x] On success, optimistic message is replaced with the server response
- [x] On failure, message shows "failed" status with retry option

**Test Cases:**
| # | Test Case | Steps | Expected Result |
|---|-----------|-------|-----------------|
| 1 | Optimistic voice message | Record and send voice message | Audio bubble appears immediately at 60% opacity |
| 2 | Upload completes | Wait for upload to finish | Opacity transitions to 100%, real message ID assigned |
| 3 | Upload fails | Record voice when server is down | "Failed" badge shown on the voice message |

### 8.11 Cookie-Restored Invite Access

#### Story: Accepting an Invite from a Reopened Browser Tab
**As a** logged-in user reopening an invite link in a new tab
**I want to** use my restored browser session without an extra login step
**So that** the invite page loads reliably even when no tab-local token exists

**Acceptance Criteria:**
- [x] Session restore from the auth cookie succeeds when sessionStorage is empty
- [x] Invite info loads correctly after cookie-based session restore
- [x] The web client does not send a synthetic placeholder bearer token on invite API requests
- [x] The invite title is visible after session restoration

### 8.12 Stable Avatar Delivery After Updates

#### Story: Seeing a Newly Updated Avatar Without Broken Images
**As a** user changing an avatar
**I want to** see the new avatar continue loading through the stable public avatar route
**So that** a successful save is not followed by a `404`

**Acceptance Criteria:**
- [x] Avatar values are stored canonically as `/files/...` on the backend
- [x] Public rendering still uses `/avatar-files/...`
- [x] Legacy `/avatars/...` and `/avatar-files/...` values remain normalizable/resolvable
- [x] Avatar delivery remains cacheable and stable after profile or bot updates

### 8.13 Context Card Fetch Discipline

#### Story: Viewing Conversation Context Without Refetch Loops
**As a** user reading a conversation
**I want to** see memory and roadmap summaries without repeated background fetches
**So that** the interface remains responsive and backend load stays bounded

**Acceptance Criteria:**
- [x] The conversation context card fetches memories and tasks once per conversation/token pair
- [x] Prompt-only rerenders do not trigger duplicate network fetches
- [x] Cached context snapshots can still hydrate before the live fetch settles

### 8.14 Safer Build Refresh Detection

#### Story: Refreshing Only for Materially Newer Web Builds
**As a** web user
**I want to** see a refresh prompt only when the deployed build meaningfully changed
**So that** I do not get stuck in a stale "newer build available" loop for same-commit rebuilds

**Acceptance Criteria:**
- [x] Build drift is detected when the app version or commit changes
- [x] Build time differences alone do not force the stale-build warning
- [x] Hard reload still unregisters service workers and clears caches before redirecting

### 8.15 Hardened Token Rotation

#### Story: Rotating a Bot Token Safely Under Repeated User Actions
**As a** bot owner
**I want to** rotate a bot token safely even if the action is triggered repeatedly
**So that** only one current permanent key survives and conflict-prone behavior is reduced

**Acceptance Criteria:**
- [x] Token rotation remains owner-only
- [x] Previous permanent keys are revoked after a successful rotation
- [x] Rotation attempts are serialized per entity on the backend
- [x] Regression coverage exists for repeated rotations and related bot update paths

---

## Future User Stories (Roadmap)

### Voice & Video
- Voice message support
- Video message support
- Real-time voice/video calls
- Screen sharing capability
- Recording and transcription

### AI Enhancements
- Smart message suggestions
- Automatic task extraction
- Sentiment analysis
- Language translation
- Content summarization

### Enterprise Features
- SSO/SAML authentication
- Audit logging and compliance
- Data residency options
- Private cloud deployment
- Advanced role management
- SLA guarantees

### Developer Experience
- Visual bot builder
- Testing framework
- Analytics dashboard
- A/B testing tools
- Version management
- Marketplace for bots

---

## Conclusion

These user stories provide comprehensive coverage of the Agent-Native IM platform capabilities, serving as both documentation and acceptance criteria for ongoing development. Each story is designed to be independently valuable while contributing to the overall platform vision of seamless human-AI collaboration.
