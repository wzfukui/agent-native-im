package main

import (
	"context"
	"log/slog"
	"os"
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

	if err := st.SeedAdmin(context.Background(), cfg.AdminUser, cfg.AdminPass); err != nil {
		slog.Error("failed to seed admin", "error", err)
		os.Exit(1)
	}

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
			pushSender.SendToEntity(context.Background(), entityID, push.Payload{
				Title:          senderName,
				Body:           body,
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
	defer cleanupCancel()
	maxAge := time.Duration(cfg.FileRetentionDays) * 24 * time.Hour
	utils.SafeGo("file-cleanup", func() {
		filestore.RunFileCleanup(cleanupCtx, st, "data/files", 24*time.Hour, maxAge, 1*time.Minute)
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

	slog.Info("Agent-Native IM server starting", "port", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
