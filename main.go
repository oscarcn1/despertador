package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"despertador/internal/alarm"
	"despertador/internal/player"
	"despertador/internal/web"
)

func main() {
	port := flag.String("port", "8080", "HTTP server port")
	configPath := flag.String("config", "/home/oscar/Projects/despertador/config.json", "Config file path")
	flag.Parse()

	cfg := alarm.NewConfig(*configPath)
	if err := cfg.Load(); err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	alarms := cfg.GetAlarms()
	log.Printf("Loaded %d alarm(s)", len(alarms))
	for _, a := range alarms {
		log.Printf("  [%s] %s: %s, enabled=%v, order=%s", a.ID, a.Name, a.TimeString(), a.Enabled, a.PlayOrder)
	}

	p := player.New()
	scheduler := alarm.NewScheduler(cfg, p)
	go scheduler.Start()

	srv := web.NewServer(cfg, scheduler)
	handler := srv.SetupRoutes()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		scheduler.Stop()
		p.Stop()
		os.Exit(0)
	}()

	addr := ":" + *port
	log.Printf("Web server listening on http://0.0.0.0%s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
