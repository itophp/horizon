package config

import (
	"baseline/services/api/internal/governance"
	"encoding/base64"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Address, DatabaseURL, DemoToken string
	SecureCookies                   bool
	Governance                      governance.Options
}

func Load() (Config, error) {
	c := Config{Address: os.Getenv("API_ADDR"), DatabaseURL: os.Getenv("DATABASE_URL"), DemoToken: os.Getenv("DEMO_AUTH_TOKEN")}
	if c.Address == "" {
		c.Address = "127.0.0.1:8080"
	}
	if raw := os.Getenv("COOKIE_SECURE"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return c, errors.New("COOKIE_SECURE must be boolean")
		}
		c.SecureCookies = parsed
	}
	host, _, err := net.SplitHostPort(c.Address)
	if err != nil {
		return c, err
	}
	ip := net.ParseIP(host)
	if c.DemoToken != "" && (ip == nil || !ip.IsLoopback()) {
		return c, errors.New("development auth requires a literal loopback listener")
	}
	if (ip == nil || !ip.IsLoopback()) && !c.SecureCookies {
		return c, errors.New("non-loopback deployments require COOKIE_SECURE=true and HTTPS")
	}

	c.Governance.StorageRoot = os.Getenv("FILE_STORAGE_ROOT")
	if c.Governance.StorageRoot == "" {
		if ip == nil || !ip.IsLoopback() {
			return c, errors.New("FILE_STORAGE_ROOT required")
		}
		root, e := filepath.Abs(".local/storage")
		if e != nil {
			return c, e
		}
		c.Governance.StorageRoot = root
	}
	if !filepath.IsAbs(c.Governance.StorageRoot) {
		return c, errors.New("FILE_STORAGE_ROOT must be absolute")
	}
	if raw := os.Getenv("SETTINGS_ENCRYPTION_KEY"); raw != "" {
		key, e := base64.StdEncoding.DecodeString(raw)
		if e != nil || len(key) != 32 {
			return c, errors.New("SETTINGS_ENCRYPTION_KEY must be base64 encoded 32 bytes")
		}
		c.Governance.EncryptionKey = key
	} else if ip == nil || !ip.IsLoopback() {
		return c, errors.New("SETTINGS_ENCRYPTION_KEY required")
	}
	c.Governance.PublicWorkspace = os.Getenv("PUBLIC_WORKSPACE_ID")
	c.Governance.PublicURL = os.Getenv("PUBLIC_APP_URL")
	if c.SecureCookies {
		origin, e := url.Parse(c.Governance.PublicURL)
		if e != nil || origin.Scheme != "https" || origin.Hostname() == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") {
			return c, errors.New("secure deployments require PUBLIC_APP_URL as an HTTPS origin")
		}
	}
	for _, host := range strings.Split(os.Getenv("SMTP_ALLOWED_HOSTS"), ",") {
		if h := strings.TrimSpace(host); h != "" {
			c.Governance.SMTPHosts = append(c.Governance.SMTPHosts, h)
		}
	}
	return c, nil
}
