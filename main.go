package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"PawonWarga-BE/docs"
	"PawonWarga-BE/internal/config"
	"PawonWarga-BE/internal/server"
)

// @title           PawonWarga API
// @version         1.0
// @description     PawonWarga Backend API Service
// @host            localhost:8080
// @BasePath        /api/v1
// @securityDefinitions.basic  BasicAuth
// @securityDefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
// @description                Enter: Bearer {token}
// @securityDefinitions.apikey ApiKeyAuth
// @in                         header
// @name                       X-API-Key
// @description                Shared secret for the internal ingest routes (Python labeling worker)
func main() {
	cfg := config.Load()
	docs.SwaggerInfo.Host = cfg.Server.SwaggerHost

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("failed to initialize server: %v", err)
	}

	go func() {
		if err := srv.Run(); err != nil {
			log.Printf("server error: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	if err := srv.Shutdown(); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
}
