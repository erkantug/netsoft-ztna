package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	ListenAddr string `yaml:"listen_addr"`
	DataDir    string `yaml:"data_dir"`
	StackDir   string `yaml:"stack_dir"`
	WebDir     string `yaml:"web_dir"`
	CertDir    string `yaml:"cert_dir"`
	LogDir     string `yaml:"log_dir"`
}

func Default() *Config {
	return &Config{
		ListenAddr: ":8080",
		DataDir:    "/opt/netsoft/data",
		StackDir:   "/opt/netsoft/stack",
		WebDir:     "/opt/netsoft/web",
		CertDir:    "/etc/netsoft/certs",
		LogDir:     "/var/log/netsoft",
	}
}

func Load(cfg *Config) error {
	// Override from env vars if set
	if v := os.Getenv("NETSOFT_LISTEN"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("NETSOFT_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("NETSOFT_STACK_DIR"); v != "" {
		cfg.StackDir = v
	}
	if v := os.Getenv("NETSOFT_WEB_DIR"); v != "" {
		cfg.WebDir = v
	}
	if v := os.Getenv("NETSOFT_CERT_DIR"); v != "" {
		cfg.CertDir = v
	}

	// Ensure directories exist
	for _, d := range []string{cfg.DataDir, cfg.StackDir, cfg.CertDir, cfg.LogDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) StackFile(name string) string {
	return filepath.Join(c.StackDir, name)
}

func (c *Config) DataFile(name string) string {
	return filepath.Join(c.DataDir, name)
}

func (c *Config) CertFile(name string) string {
	return filepath.Join(c.CertDir, name)
}

func (c *Config) LogFile(name string) string {
	return filepath.Join(c.LogDir, name)
}
