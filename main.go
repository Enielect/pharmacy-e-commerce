package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pharmacy/internal/admin"
	"pharmacy/internal/auth"
	"pharmacy/internal/cart"
	"pharmacy/internal/catalog"
	"pharmacy/internal/config"
	"pharmacy/internal/db"
	"pharmacy/internal/handlers"
	"pharmacy/internal/imagestore"
	"pharmacy/internal/logger"
	"pharmacy/internal/orders"
	"pharmacy/internal/payments"

	"github.com/go-chi/chi/v5"
)

//go:embed templates
var templatesFS embed.FS

func main() {
	cfg := config.Load()
	log := logger.New()

	log.Info("starting pharmacy e-commerce platform")

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.RunMigrations(database, "migrations"); err != nil {
		log.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	authStore := auth.NewStore(database)
	catalogStore := catalog.NewStore(database)
	cartStore := cart.NewStore(database)
	orderStore := orders.NewStore(database)
	paymentProv := payments.NewStubProvider(database)
	imageStore := imagestore.NewLocalStore(cfg.UploadDir, "/uploads")

	// Seed admin user if not exists
	seedAdmin(authStore, log)

	// Seed product drafts for admin demo
	seedDrafts(admin.NewStore(database), log)

	h := handlers.New(cfg, log, authStore, catalogStore, cartStore, orderStore, paymentProv, imageStore, templatesFS)

	r := chi.NewRouter()
	h.SetupRoutes(r)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("listening", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

func seedAdmin(authStore *auth.Store, log *slog.Logger) {
	_, err := authStore.CreateUser("admin@pharmacy.com", "admin123", "admin")
	if err != nil {
		log.Info("admin user may already exist", "error", err)
	}
	_, err = authStore.CreateUser("pharmacist@pharmacy.com", "pharm123", "pharmacist")
	if err != nil {
		log.Info("pharmacist user may already exist", "error", err)
	}
	_, err = authStore.CreateUser("customer@test.com", "customer123", "customer")
	if err != nil {
		log.Info("test customer may already exist", "error", err)
	}
}

func seedDrafts(adminStore *admin.Store, log *slog.Logger) {
	drafts, err := adminStore.ListDrafts("")
	if err != nil || len(drafts) > 0 {
		return
	}

	// Insert some sample drafts
	db := adminStore.DB()
	for i := 1; i <= 5; i++ {
		draftName := fmt.Sprintf("New Product Draft %d", i)
		slug := fmt.Sprintf("new-product-draft-%d", i)
		_, err := db.Exec(`
			INSERT INTO product_drafts (image_key, suggested_fields, status)
			VALUES ($1, $2, 'pending_review')
		`, "", fmt.Sprintf(`{"name":"%s","slug":"%s"}`, draftName, slug))
		if err != nil {
			log.Error("seed draft", "error", err)
		}
	}
	log.Info("seeded product drafts for admin review")
}
