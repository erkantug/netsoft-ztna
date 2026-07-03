package deploy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/netsoft/ztna-wizard/internal/config"
	"github.com/netsoft/ztna-wizard/internal/db"
)

type Deployer struct {
	cfg *config.Config
}

func New(cfg *config.Config) *Deployer {
	return &Deployer{cfg: cfg}
}

func (d *Deployer) Deploy(all *db.AllData, logCh chan string) error {
	logCh <- "Generating database password..."
	dbPass, err := generatePassword(32)
	if err != nil {
		return fmt.Errorf("generate password: %w", err)
	}

	composeData := ComposeDataFromWizard(all, dbPass)

	logCh <- "Generating docker-compose.yml..."
	compose, err := GenerateCompose(composeData)
	if err != nil {
		return fmt.Errorf("generate compose: %w", err)
	}

	composePath := d.cfg.StackFile("docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(compose), 0644); err != nil {
		return fmt.Errorf("write compose: %w", err)
	}

	logCh <- "Generating .env..."
	env, err := GenerateEnv(composeData)
	if err != nil {
		return fmt.Errorf("generate env: %w", err)
	}

	envPath := d.cfg.StackFile(".env")
	if err := os.WriteFile(envPath, []byte(env), 0600); err != nil {
		return fmt.Errorf("write env: %w", err)
	}

	logCh <- "Starting Docker Compose stack..."
	cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d")
	cmd.Dir = d.cfg.StackDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose up failed: %s: %w", string(output), err)
	}
	logCh <- string(output)

	logCh <- "Stack deployed successfully!"
	return nil
}

func generatePassword(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (d *Deployer) EnsurePaths() error {
	dirs := []string{
		d.cfg.CertDir,
		filepath.Join(d.cfg.DataDir, "postgres"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	return nil
}

func HealthCheck() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker not found: %w", err)
	}
	cmd := exec.Command("docker", "compose", "version")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker compose not available: %s: %w", string(out), err)
	}
	log.Println("Docker Compose health check passed")
	return nil
}
