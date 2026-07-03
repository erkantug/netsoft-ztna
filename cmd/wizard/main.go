package main

import (
	"log"
	"net/http"
	"path/filepath"

	"github.com/netsoft/ztna-wizard/internal/api"
	"github.com/netsoft/ztna-wizard/internal/config"
	"github.com/netsoft/ztna-wizard/internal/db"
	"github.com/netsoft/ztna-wizard/internal/deploy"
	"github.com/netsoft/ztna-wizard/internal/tls"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Netsoft ZTNA Wizard v1.0.0 starting...")

	// Configuration
	cfg := config.Default()
	if err := config.Load(cfg); err != nil {
		log.Fatalf("Config error: %v", err)
	}

	// Database
	database, err := db.New(filepath.Join(cfg.DataDir, "wizard.db"))
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}
	defer database.Close()

	// Check if already deployed
	deployed, err := database.IsDeployed()
	if err != nil {
		log.Printf("Warning: could not check deploy status: %v", err)
	}
	if deployed {
		log.Println("Already deployed. Wizard is disabled.")
		log.Println("To re-run, delete /opt/netsoft/data/wizard.db")
		return
	}

	// Docker health check
	if err := deploy.HealthCheck(); err != nil {
		log.Printf("Warning: Docker check failed: %v", err)
	}

	// Create deployer and TLS manager
	deployer := deploy.New(cfg)
	tlsMgr := tls.New(cfg)

	// Ensure required dirs
	if err := deployer.EnsurePaths(); err != nil {
		log.Printf("Warning: could not create paths: %v", err)
	}

	// API server
	mux := http.NewServeMux()
	apiHandler := api.New(cfg, database, deployer, tlsMgr)
	apiHandler.RegisterRoutes(mux)

	// Serve web UI
	fs := http.FileServer(http.Dir(cfg.WebDir))
	mux.Handle("GET /static/", fs)
	mux.HandleFunc("GET /", apiHandler.ServeWizard)

	addr := cfg.ListenAddr
	log.Printf("========================================")
	log.Printf("  Netsoft ZTNA Wizard")
	log.Printf("  Open: http://localhost%s", addr)
	log.Printf("========================================")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
