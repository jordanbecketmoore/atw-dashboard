package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jordanm/atw-dashboard/internal/config"
	"github.com/jordanm/atw-dashboard/internal/httpapi"
	"github.com/jordanm/atw-dashboard/internal/hub"
	"github.com/jordanm/atw-dashboard/internal/leaderboard"
	"github.com/jordanm/atw-dashboard/internal/warrior"
)

func main() {
	configPath := flag.String("config", "/etc/atw-dashboard/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	log.Printf("loaded config: %d warriors, nickname=%s, listen=%s", len(cfg.Warriors), cfg.Nickname, cfg.ListenAddr)

	h := hub.New(cfg.Nickname)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	for _, w := range cfg.Warriors {
		w := w
		wg.Add(1)
		go func() {
			defer wg.Done()
			warrior.Run(ctx, w, cfg, h)
		}()
	}

	poller := leaderboard.New(h, cfg.Nickname, cfg.LeaderboardInterval)
	wg.Add(1)
	go func() {
		defer wg.Done()
		poller.Run(ctx)
	}()

	mux := http.NewServeMux()
	httpapi.NewServer(h).Register(mux)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case <-ctx.Done():
		log.Printf("shutdown signal received")
	case err := <-serverErr:
		if err != nil {
			log.Printf("server error: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}

	stop()
	wg.Wait()
	log.Printf("bye")
}
