package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/netsoft/ztna-wizard/internal/config"
	"github.com/netsoft/ztna-wizard/internal/db"
	"github.com/netsoft/ztna-wizard/internal/deploy"
	"github.com/netsoft/ztna-wizard/internal/tls"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	cfg      *config.Config
	db       *db.WizardDB
	deployer *deploy.Deployer
	tlsMgr   *tls.TLSManager
	tmpl     *template.Template
	mu       sync.Mutex
	logBuf   []string
	logSubs  []chan string
}

func New(cfg *config.Config, database *db.WizardDB, deployer *deploy.Deployer, tlsMgr *tls.TLSManager) *Handler {
	return &Handler{
		cfg:      cfg,
		db:       database,
		deployer: deployer,
		tlsMgr:   tlsMgr,
		logSubs:  make([]chan string, 0),
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/status", h.handleStatus)
	mux.HandleFunc("GET /api/step/{step}", h.handleGetStep)
	mux.HandleFunc("POST /api/step/{step}", h.handleSaveStep)
	mux.HandleFunc("POST /api/deploy", h.handleDeploy)
	mux.HandleFunc("GET /api/deploy/log", h.handleDeployLog)
	mux.HandleFunc("GET /api/health", h.handleHealth)
}

func (h *Handler) ServeWizard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	deployed, _ := h.db.IsDeployed()
	currentStep, _ := h.db.GetCurrentStep()

	data := map[string]interface{}{
		"Deployed":    deployed,
		"CurrentStep": int(currentStep),
	}

	tmpl := template.Must(template.ParseFiles(
		filepath.Join(h.cfg.WebDir, "templates", "layout.html"),
		filepath.Join(h.cfg.WebDir, "templates", "index.html"),
	))

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	deployed, _ := h.db.IsDeployed()
	currentStep, _ := h.db.GetCurrentStep()
	all, _ := h.db.GetAllData()

	respondJSON(w, map[string]interface{}{
		"deployed":     deployed,
		"current_step": int(currentStep),
		"domain":       h.getDomain(all),
	})
}

func (h *Handler) handleGetStep(w http.ResponseWriter, r *http.Request) {
	step := parseStep(r.PathValue("step"))
	all, err := h.db.GetAllData()
	if err != nil {
		respondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var data interface{}
	switch step {
	case db.StepNetwork:
		data = all.Network
	case db.StepTLS:
		data = all.TLS
	case db.StepAdmin:
		data = all.Admin
	case db.StepIDP:
		data = all.IDP
	default:
		respondError(w, "invalid step", http.StatusBadRequest)
		return
	}
	respondJSON(w, data)
}

func (h *Handler) handleSaveStep(w http.ResponseWriter, r *http.Request) {
	step := parseStep(r.PathValue("step"))

	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		respondError(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	var data interface{}
	switch step {
	case db.StepNetwork:
		d := &db.NetworkData{}
		if err := json.Unmarshal(raw, d); err != nil {
			respondError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if d.Domain == "" {
			respondError(w, "domain is required", http.StatusBadRequest)
			return
		}
		data = d

	case db.StepTLS:
		d := &db.TLSData{}
		if err := json.Unmarshal(raw, d); err != nil {
			respondError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if d.Method == "" {
			respondError(w, "TLS method is required", http.StatusBadRequest)
			return
		}
		data = d

	case db.StepAdmin:
		d := &db.AdminData{}
		if err := json.Unmarshal(raw, d); err != nil {
			respondError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if d.Username == "" || d.Password == "" {
			respondError(w, "username and password required", http.StatusBadRequest)
			return
		}
		// Hash password before storing
		hash, err := bcrypt.GenerateFromPassword([]byte(d.Password), bcrypt.DefaultCost)
		if err != nil {
			respondError(w, "failed to hash password", http.StatusInternalServerError)
			return
		}
		d.Password = string(hash)
		data = d

	case db.StepIDP:
		d := &db.IDPData{}
		if err := json.Unmarshal(raw, d); err != nil {
			respondError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if d.Provider == "" {
			d.Provider = "local"
		}
		data = d

	default:
		respondError(w, "invalid step", http.StatusBadRequest)
		return
	}

	if err := h.db.SaveStepData(step, data); err != nil {
		respondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleDeploy(w http.ResponseWriter, r *http.Request) {
	all, err := h.db.GetAllData()
	if err != nil {
		respondError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if all.Network == nil || all.Admin == nil {
		respondError(w, "complete all steps first", http.StatusBadRequest)
		return
	}

	// Handle TLS
	if all.TLS != nil {
		switch all.TLS.Method {
		case "selfsigned":
			if err := h.tlsMgr.GenerateSelfSigned(all.Network.Domain); err != nil {
				respondError(w, fmt.Sprintf("TLS error: %v", err), http.StatusInternalServerError)
				return
			}
		case "custom":
			if err := h.tlsMgr.SaveCustomCert(all.TLS.CertPEM, all.TLS.KeyPEM); err != nil {
				respondError(w, fmt.Sprintf("cert save error: %v", err), http.StatusInternalServerError)
				return
			}
		}
	}

	// Start deploy in background
	logCh := make(chan string, 100)
	go func() {
		h.mu.Lock()
		h.logBuf = nil
		h.mu.Unlock()

		if err := h.deployer.EnsurePaths(); err != nil {
			logCh <- fmt.Sprintf("ERROR: %v", err)
			close(logCh)
			return
		}

		if err := h.deployer.Deploy(all, logCh); err != nil {
			logCh <- fmt.Sprintf("ERROR: %v", err)
			close(logCh)
			return
		}

		h.db.MarkDeployed()
		close(logCh)
	}()

	// Stream logs to subscribers
	go func() {
		for msg := range logCh {
			h.mu.Lock()
			h.logBuf = append(h.logBuf, msg)
			for _, ch := range h.logSubs {
				select {
				case ch <- msg:
				default:
				}
			}
			h.mu.Unlock()
		}
		h.mu.Lock()
		for _, ch := range h.logSubs {
			close(ch)
		}
		h.logSubs = nil
		h.mu.Unlock()
	}()

	respondJSON(w, map[string]string{"status": "deploying"})
}

func (h *Handler) handleDeployLog(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 50)
	h.mu.Lock()
	h.logSubs = append(h.logSubs, ch)
	// Send existing buffered logs
	for _, msg := range h.logBuf {
		fmt.Fprintf(w, "data: %s\n\n", msg)
	}
	flusher.Flush()
	h.mu.Unlock()

	notify := r.Context().Done()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				fmt.Fprintf(w, "event: done\ndata: \n\n")
				flusher.Flush()
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-notify:
			return
		}
	}
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	deployed, _ := h.db.IsDeployed()
	respondJSON(w, map[string]interface{}{
		"status":   "ok",
		"deployed": deployed,
	})
}

// Helpers

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func parseStep(s string) db.Step {
	switch s {
	case "1":
		return db.StepNetwork
	case "2":
		return db.StepTLS
	case "3":
		return db.StepAdmin
	case "4":
		return db.StepIDP
	default:
		return db.StepNetwork
	}
}

func (h *Handler) getDomain(all *db.AllData) string {
	if all != nil && all.Network != nil {
		return all.Network.Domain
	}
	return ""
}

// Ensure template parsing works - embed function
func init() {
	log.SetPrefix("[netsoft-wizard] ")
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
}
