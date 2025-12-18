package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auth-service/internal/auth"
	httpapi "auth-service/internal/http"
)

func main() {
	addr := ":8082"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}

	userSvcURL := httpapi.MustEnv("USER_SERVICE_URL")
	jwtSecret := httpapi.MustEnv("JWT_SECRET")

	handlers := httpapi.Handlers{
		UserClient: auth.UserServiceClient{
			BaseURL: userSvcURL,
			Client:  auth.DefaultHTTPClient(),
		},
		JWTSecret: jwtSecret,
		JWTTTL:    30 * time.Minute,
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: httpapi.Router(handlers),
	}

	go func() {
		log.Printf("auth-service listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Println("auth-service stopped")
}
