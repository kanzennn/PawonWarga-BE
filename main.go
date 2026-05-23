package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "PawonWarga-BE/docs" // swagger docs — regenerate with: swag init
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
func main() {
	cfg := config.Load()

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
