package main

import (
	"context"
	"log"

	"github.com/wzfukui/agent-native-im/internal/config"
	"github.com/wzfukui/agent-native-im/internal/handler"
	"github.com/wzfukui/agent-native-im/internal/store/postgres"
	"github.com/wzfukui/agent-native-im/internal/webhook"
	"github.com/wzfukui/agent-native-im/internal/ws"
)

func main() {
	cfg := config.Load()

	st, err := postgres.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer st.Close()

	if err := st.SeedAdmin(context.Background(), cfg.AdminUser, cfg.AdminPass); err != nil {
		log.Fatalf("failed to seed admin: %v", err)
	}

	wh := webhook.NewDeliverer(st)
	hub := ws.NewHub(st)
	go hub.Run()

	srv := &handler.Server{
		Config:  cfg,
		Store:   st,
		Hub:     hub,
		Webhook: wh,
		Auth:    &handler.AuthHelper{Secret: cfg.JWTSecret},
	}

	r := handler.NewRouter(srv)

	log.Printf("Agent-Native IM server starting on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
