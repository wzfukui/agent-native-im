package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wzfukui/agent-native-im/internal/config"
	"github.com/wzfukui/agent-native-im/internal/filestore"
	"github.com/wzfukui/agent-native-im/internal/handler"
	"github.com/wzfukui/agent-native-im/internal/model"
	"github.com/wzfukui/agent-native-im/internal/push"
	"github.com/wzfukui/agent-native-im/internal/store/postgres"
	"github.com/wzfukui/agent-native-im/internal/utils"
	"github.com/wzfukui/agent-native-im/internal/webhook"
	"github.com/wzfukui/agent-native-im/internal/ws"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	st, err := postgres.New(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	wh := webhook.NewDeliverer(st)
	hub := ws.NewHub(st)

	// Setup push notifications
	pushSender := push.NewSender(st, cfg)
	if pushSender != nil {
		hub.OnPush = func(entityID int64, msg *model.Message) {
			senderName := "Someone"
			if msg.Sender != nil {
				if msg.Sender.DisplayName != "" {
					senderName = msg.Sender.DisplayName
				} else {
					senderName = msg.Sender.Name
				}
			}
			body := ""
			if msg.Layers.Summary != "" {
				body = msg.Layers.Summary
			}
			path := "/chat/" + fmt.Sprint(msg.ConversationID)
			if conv, err := st.GetConversation(context.Background(), msg.ConversationID); err == nil {
				if publicID := conversationPublicID(conv); publicID != "" {
					path = "/chat/public/" + publicID
				}
			}
			pushSender.SendToEntity(context.Background(), entityID, push.Payload{
				Title:          senderName,
				Body:           body,
				Kind:           "message.new",
				Path:           path,
				ConversationID: msg.ConversationID,
				MessageID:      msg.ID,
			})
		}
		slog.Info("push: Web Push notifications enabled")
	}

	go hub.Run()

	fs, err := filestore.NewLocalStore("data/files", "/files")
	if err != nil {
		slog.Error("failed to init file store", "error", err)
		os.Exit(1)
	}

	// Start file cleanup goroutine (1-min startup delay, then every 24h)
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	maxAge := time.Duration(cfg.FileRetentionDays) * 24 * time.Hour
	utils.SafeGo("file-cleanup", func() {
		filestore.RunFileCleanup(cleanupCtx, st, "data/files", 24*time.Hour, maxAge, 1*time.Minute)
	})

	// Start periodic DB pool stats logging
	poolCtx, poolCancel := context.WithCancel(context.Background())
	sqldb := st.DB.DB
	utils.SafeGo("db-pool-monitor", func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-poolCtx.Done():
				return
			case <-ticker.C:
				stats := sqldb.Stats()
				slog.Info("db pool stats",
					"max_open", stats.MaxOpenConnections,
					"open", stats.OpenConnections,
					"in_use", stats.InUse,
					"idle", stats.Idle,
					"wait_count", stats.WaitCount,
					"wait_duration_ms", stats.WaitDuration.Milliseconds(),
					"max_idle_closed", stats.MaxIdleClosed,
					"max_lifetime_closed", stats.MaxLifetimeClosed,
				)
			}
		}
	})

	srv := &handler.Server{
		Config:    cfg,
		Store:     st,
		Hub:       hub,
		Webhook:   wh,
		Auth:      &handler.AuthHelper{Secret: cfg.JWTSecret, TokenTTL: time.Duration(cfg.JWTTTLHours) * time.Hour},
		FileStore: fs,
		Push:      pushSender,
	}

	r := handler.NewRouter(srv)

	httpSrv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("Agent-Native IM server starting", "port", cfg.Port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for SIGTERM or SIGINT
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit
	slog.Info("shutting down gracefully...", "signal", sig.String())

	// Give in-flight requests 15 seconds to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	// Cancel background goroutines
	cleanupCancel()
	poolCancel()

	// Close database connection (deferred st.Close() will also run)
	slog.Info("shutdown complete")
}

func conversationPublicID(conv *model.Conversation) string {
	if conv == nil || len(conv.Metadata) == 0 {
		return ""
	}
	var meta map[string]any
	if err := json.Unmarshal(conv.Metadata, &meta); err != nil {
		return ""
	}
	raw, _ := meta["public_id"].(string)
	return raw
}
