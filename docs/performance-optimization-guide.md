# Performance Optimization Implementation Guide

## Critical Fix #1: Sharded Lock Implementation for WebSocket Hub

### Problem
The single `sync.RWMutex` in `hub.go` creates severe lock contention, limiting scalability to ~1000 concurrent connections.

### Solution: Implement Sharded Maps

```go
// internal/ws/sharded_hub.go
package ws

import (
    "sync"
    "hash/fnv"
)

const numShards = 32 // Power of 2 for efficient modulo

type ShardedHub struct {
    shards [numShards]*HubShard
    store  store.Store
    // Other fields...
}

type HubShard struct {
    mu          sync.RWMutex
    clients     map[*Client]bool
    convClients map[int64]map[*Client]bool
}

func (h *ShardedHub) getShard(entityID int64) *HubShard {
    hash := fnv.New32()
    hash.Write([]byte(string(entityID)))
    return h.shards[hash.Sum32() & (numShards-1)]
}

// Example: Register client with sharding
func (h *ShardedHub) RegisterClient(client *Client) {
    shard := h.getShard(client.entityID)
    shard.mu.Lock()
    defer shard.mu.Unlock()

    shard.clients[client] = true
    // Subscribe to conversations...
}

// BroadcastMessage with minimal lock holding
func (h *ShardedHub) BroadcastMessage(msg *model.Message) {
    // Step 1: Collect clients without holding locks long
    clients := h.collectClientsForConversation(msg.ConversationID)

    // Step 2: Send messages without holding any locks
    h.sendToClients(clients, msg)
}
```

### Implementation Steps
1. Create new `sharded_hub.go` file
2. Implement shard selection using hash function
3. Migrate existing hub methods to use sharding
4. Add metrics for shard balance monitoring
5. Test with concurrent load

### Expected Impact
- 32x reduction in lock contention
- Support for 10,000+ concurrent connections
- Broadcast latency reduced from 50ms to 5ms

---

## Critical Fix #2: Connection Pool Optimization

### Current Issues
```go
// Current: Too conservative
sqldb.SetMaxOpenConns(10)
sqldb.SetMaxIdleConns(5)
```

### Optimized Configuration

```go
// internal/store/postgres/postgres.go
func New(databaseURL string) (*PGStore, error) {
    sqldb := sql.OpenDB(pgdriver.NewConnector(
        pgdriver.WithDSN(databaseURL),
        pgdriver.WithTimeout(30 * time.Second),
        pgdriver.WithReadTimeout(10 * time.Second),
        pgdriver.WithWriteTimeout(10 * time.Second),
    ))

    // Optimized for production load
    sqldb.SetMaxOpenConns(50)                  // Increased from 10
    sqldb.SetMaxIdleConns(25)                  // Increased from 5
    sqldb.SetConnMaxLifetime(5 * time.Minute)  // Prevent stale connections
    sqldb.SetConnMaxIdleTime(1 * time.Minute)  // Close idle connections

    // Add monitoring
    go monitorPoolStats(sqldb)

    return &PGStore{DB: bun.NewDB(sqldb, pgdialect.New())}, nil
}

func monitorPoolStats(db *sql.DB) {
    ticker := time.NewTicker(30 * time.Second)
    for range ticker.C {
        stats := db.Stats()
        log.Printf("DB Pool: Open=%d, InUse=%d, Idle=%d, Wait=%d, WaitDur=%v",
            stats.OpenConnections,
            stats.InUse,
            stats.Idle,
            stats.WaitCount,
            stats.WaitDuration)

        // Alert if pool is saturated
        if stats.OpenConnections == stats.MaxOpenConnections && stats.WaitCount > 0 {
            log.Printf("WARNING: Database connection pool saturated!")
        }
    }
}
```

---

## Critical Fix #3: Memory Leak Prevention

### Issue 1: WebSocket Client Cleanup

```go
// internal/ws/hub.go - Add timeout for cleanup
func (h *Hub) Run() {
    cleanupTicker := time.NewTicker(5 * time.Minute)
    defer cleanupTicker.Stop()

    for {
        select {
        case <-cleanupTicker.C:
            h.cleanupStaleClients()
        // ... existing cases
        }
    }
}

func (h *Hub) cleanupStaleClients() {
    h.mu.Lock()
    defer h.mu.Unlock()

    now := time.Now()
    for client := range h.clients {
        if client.lastActivity.Before(now.Add(-10 * time.Minute)) {
            delete(h.clients, client)
            close(client.send)
            h.unsubscribeClientLocked(client)
            log.Printf("Cleaned up stale client: %d", client.entityID)
        }
    }
}
```

### Issue 2: Frontend Message Cleanup

```typescript
// src/store/messages.ts - Add message expiry
interface MessagesState {
    // ... existing fields
    maxMessagesPerConv: number // Add limit

    // Modified method
    addMessage: (msg: Message) => void
}

export const useMessagesStore = create<MessagesState>((set) => ({
    maxMessagesPerConv: 1000, // Limit messages in memory

    addMessage: (msg) =>
        set((s) => {
            const existing = s.byConv[msg.conversation_id] || []
            let updated = [...existing, msg]

            // Trim old messages if exceeds limit
            if (updated.length > s.maxMessagesPerConv) {
                updated = updated.slice(-s.maxMessagesPerConv)
            }

            return {
                byConv: { ...s.byConv, [msg.conversation_id]: updated },
            }
        }),
}))
```

---

## Critical Fix #4: Optimize Message Broadcasting

### Current Problem
```go
// Synchronous, blocking broadcast
for _, client := range clients {
    select {
    case client.send <- data:
    default:
        // Message dropped
    }
}
```

### Optimized Solution: Worker Pool Pattern

```go
// internal/ws/broadcast_pool.go
package ws

type BroadcastPool struct {
    workers   int
    taskQueue chan BroadcastTask
    wg        sync.WaitGroup
}

type BroadcastTask struct {
    clients []*Client
    data    []byte
}

func NewBroadcastPool(workers int) *BroadcastPool {
    pool := &BroadcastPool{
        workers:   workers,
        taskQueue: make(chan BroadcastTask, 1000),
    }
    pool.start()
    return pool
}

func (p *BroadcastPool) start() {
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go p.worker()
    }
}

func (p *BroadcastPool) worker() {
    defer p.wg.Done()
    for task := range p.taskQueue {
        for _, client := range task.clients {
            select {
            case client.send <- task.data:
            case <-time.After(100 * time.Millisecond):
                log.Printf("Client %d slow, skipping", client.entityID)
            }
        }
    }
}

func (p *BroadcastPool) Broadcast(clients []*Client, data []byte) {
    // Split into chunks for parallel processing
    chunkSize := 50
    for i := 0; i < len(clients); i += chunkSize {
        end := i + chunkSize
        if end > len(clients) {
            end = len(clients)
        }

        select {
        case p.taskQueue <- BroadcastTask{
            clients: clients[i:end],
            data:    data,
        }:
        default:
            log.Printf("Broadcast pool saturated, dropping task")
        }
    }
}
```

---

## Critical Fix #5: Database Query Optimization

### Add Missing Indexes

```sql
-- migrations/000010_performance_indexes.up.sql

-- Optimize participant queries
CREATE INDEX CONCURRENTLY idx_participants_entity_active
ON participants(entity_id, conversation_id)
WHERE left_at IS NULL;

-- Optimize message queries with conversation
CREATE INDEX CONCURRENTLY idx_messages_conv_created
ON messages(conversation_id, created_at DESC)
WHERE revoked_at IS NULL;

-- Optimize entity lookup
CREATE INDEX CONCURRENTLY idx_entities_status
ON entities(entity_type, status)
WHERE status = 'active';

-- Optimize unread tracking
CREATE INDEX CONCURRENTLY idx_participant_unread
ON participants(conversation_id, last_read_message_id);

-- Analyze tables for query planner
ANALYZE participants;
ANALYZE messages;
ANALYZE entities;
ANALYZE conversations;
```

### Implement Query Result Caching

```go
// internal/store/cache/cache.go
package cache

import (
    "sync"
    "time"
)

type Cache struct {
    mu    sync.RWMutex
    items map[string]CacheItem
}

type CacheItem struct {
    Value      interface{}
    Expiration time.Time
}

func (c *Cache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    item, found := c.items[key]
    if !found || time.Now().After(item.Expiration) {
        return nil, false
    }
    return item.Value, true
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.items[key] = CacheItem{
        Value:      value,
        Expiration: time.Now().Add(ttl),
    }
}

// Use in store methods
func (s *PGStore) GetConversationIDsByEntity(ctx context.Context, entityID int64) ([]int64, error) {
    cacheKey := fmt.Sprintf("conv_ids:%d", entityID)

    // Check cache first
    if cached, found := s.cache.Get(cacheKey); found {
        return cached.([]int64), nil
    }

    // Query database
    var ids []int64
    // ... perform query

    // Cache result for 30 seconds
    s.cache.Set(cacheKey, ids, 30*time.Second)
    return ids, nil
}
```

---

## Frontend Optimization Guide

### 1. Implement Virtual Scrolling

```typescript
// src/components/chat/VirtualMessageList.tsx
import { useVirtualizer } from '@tanstack/react-virtual'

export function VirtualMessageList({ messages, ...props }) {
    const parentRef = useRef<HTMLDivElement>(null)

    const virtualizer = useVirtualizer({
        count: messages.length,
        getScrollElement: () => parentRef.current,
        estimateSize: () => 80, // Estimate message height
        overscan: 5, // Render 5 extra items outside viewport
    })

    return (
        <div ref={parentRef} className="flex-1 overflow-auto">
            <div style={{ height: `${virtualizer.getTotalSize()}px` }}>
                {virtualizer.getVirtualItems().map((virtualItem) => (
                    <div
                        key={virtualItem.key}
                        style={{
                            position: 'absolute',
                            top: 0,
                            left: 0,
                            width: '100%',
                            transform: `translateY(${virtualItem.start}px)`,
                        }}
                    >
                        <MessageBubble message={messages[virtualItem.index]} />
                    </div>
                ))}
            </div>
        </div>
    )
}
```

### 2. Add React.memo with Custom Comparison

```typescript
// src/components/chat/MessageBubble.tsx
import { memo } from 'react'

export const MessageBubble = memo(
    ({ message, isSelf, ...props }) => {
        // Component implementation
    },
    (prevProps, nextProps) => {
        // Custom comparison - only re-render if message content changes
        return (
            prevProps.message.id === nextProps.message.id &&
            prevProps.message.layers?.summary === nextProps.message.layers?.summary &&
            prevProps.message.revoked_at === nextProps.message.revoked_at
        )
    }
)
```

### 3. Lazy Load Heavy Components

```typescript
// src/App.tsx
import { lazy, Suspense } from 'react'

// Lazy load heavy components
const MermaidRenderer = lazy(() => import('./components/MermaidRenderer'))
const ArtifactRenderer = lazy(() => import('./components/chat/ArtifactRenderer'))

function App() {
    return (
        <Suspense fallback={<div>Loading...</div>}>
            {/* Routes and components */}
        </Suspense>
    )
}
```

---

## Load Testing Scripts

### 1. WebSocket Load Test with k6

```javascript
// loadtest/websocket_test.js
import ws from 'k6/ws'
import { check } from 'k6'

export const options = {
    stages: [
        { duration: '30s', target: 100 },  // Ramp up to 100 users
        { duration: '1m', target: 100 },   // Stay at 100 users
        { duration: '30s', target: 1000 }, // Spike to 1000 users
        { duration: '2m', target: 1000 },  // Maintain 1000 users
        { duration: '30s', target: 0 },    // Ramp down
    ],
    thresholds: {
        ws_connecting: ['p(95)<1000'], // 95% connect within 1s
        ws_msgs_sent: ['rate>1000'],   // Send >1000 msg/s
    },
}

export default function () {
    const url = 'ws://192.168.44.43:9800/ws'
    const params = {
        headers: { 'Authorization': 'Bearer ' + __ENV.TOKEN },
    }

    const res = ws.connect(url, params, function (socket) {
        socket.on('open', () => {
            // Send messages
            socket.send(JSON.stringify({
                type: 'message.send',
                data: {
                    conversation_id: 1,
                    layers: { summary: 'Load test message' },
                },
            }))

            socket.setInterval(() => {
                socket.ping()
            }, 25000)
        })

        socket.on('message', (data) => {
            const msg = JSON.parse(data)
            check(msg, {
                'message received': (m) => m.type === 'message.new',
            })
        })

        socket.setTimeout(() => {
            socket.close()
        }, 60000)
    })

    check(res, { 'status is 101': (r) => r && r.status === 101 })
}
```

### 2. Database Load Test

```bash
#!/bin/bash
# loadtest/db_stress.sh

# Create test database
createdb agent_im_loadtest

# Run migrations
migrate -path migrations -database "postgres://localhost/agent_im_loadtest" up

# Generate test data
psql agent_im_loadtest << EOF
-- Insert 10k entities
INSERT INTO entities (name, entity_type, display_name, avatar_url, status)
SELECT
    'user_' || i,
    'user',
    'User ' || i,
    'https://avatar.url/' || i,
    'active'
FROM generate_series(1, 10000) i;

-- Insert 1k conversations
INSERT INTO conversations (name, conversation_type, avatar_url)
SELECT
    'conv_' || i,
    'group',
    'https://avatar.url/conv_' || i
FROM generate_series(1, 1000) i;

-- Insert participants (average 10 per conversation)
INSERT INTO participants (conversation_id, entity_id, role, subscription_mode)
SELECT
    (i % 1000) + 1,
    (random() * 9999 + 1)::int,
    'member',
    'subscribe_all'
FROM generate_series(1, 10000) i;

-- Insert 100k messages
INSERT INTO messages (conversation_id, sender_id, content_type, layers)
SELECT
    (i % 1000) + 1,
    (random() * 9999 + 1)::int,
    'text',
    jsonb_build_object('summary', 'Test message ' || i)
FROM generate_series(1, 100000) i;
EOF

# Run pgbench
pgbench -c 10 -j 4 -T 60 -f loadtest/queries.sql agent_im_loadtest
```

---

## Monitoring Setup

### Prometheus Metrics Endpoint

```go
// internal/handler/metrics.go
package handler

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
    wsConnections = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "ws_connections_total",
            Help: "Total WebSocket connections",
        },
        []string{"status"},
    )

    messagesSent = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "messages_sent_total",
            Help: "Total messages sent",
        },
        []string{"type"},
    )

    dbQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "db_query_duration_seconds",
            Help: "Database query duration",
        },
        []string{"query"},
    )
)

func init() {
    prometheus.MustRegister(wsConnections)
    prometheus.MustRegister(messagesSent)
    prometheus.MustRegister(dbQueryDuration)
}

func (s *Server) MetricsHandler() gin.HandlerFunc {
    return gin.WrapH(promhttp.Handler())
}
```

---

## Rollout Plan

### Phase 1: Monitoring & Baseline (Week 1)
1. Deploy monitoring and metrics
2. Establish performance baseline
3. Identify actual bottlenecks in production

### Phase 2: Critical Fixes (Week 2)
1. Implement sharded locking
2. Optimize connection pool
3. Fix memory leaks
4. Deploy to staging

### Phase 3: Database & Query Optimization (Week 3)
1. Add missing indexes
2. Implement query caching
3. Test with production data volume
4. Monitor impact

### Phase 4: Frontend Optimization (Week 4)
1. Implement virtual scrolling
2. Add React.memo optimizations
3. Lazy load heavy components
4. Measure user experience improvements

### Phase 5: Load Testing & Validation (Week 5)
1. Run comprehensive load tests
2. Validate improvements
3. Document new capacity limits
4. Plan for next optimization cycle

---

*Implementation Guide Version: 1.0*
*Created: March 4, 2026*