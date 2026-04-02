package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

const TokenEnvVar = "CODA_API_TOKEN"
const configDirEnvVar = "CODA_CONFIG_DIR"

type AuthConfig struct {
	APIToken string `yaml:"api_token"`
}

func ConfigDir() (string, error) {
	if dir := os.Getenv(configDirEnvVar); dir != "" {
		return dir, nil
	}

	if xdgDir := os.Getenv("XDG_CONFIG_HOME"); xdgDir != "" {
		return filepath.Join(xdgDir, "coda-cli"), nil
	}

	if runtime.GOOS == "windows" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve user config dir: %w", err)
		}
		return filepath.Join(configDir, "coda-cli"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home dir: %w", err)
	}

	return filepath.Join(homeDir, ".config", "coda-cli"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func LoadAuthConfig() (*AuthConfig, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read auth config: %w", err)
	}

	var cfg AuthConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse auth config: %w", err)
	}

	return &cfg, nil
}

func SaveAuthToken(token string) (string, error) {
	path, err := ConfigPath()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", fmt.Errorf("failed to create auth config dir: %w", err)
	}

	data, err := yaml.Marshal(&AuthConfig{APIToken: token})
	if err != nil {
		return "", fmt.Errorf("failed to encode auth config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write auth config: %w", err)
	}

	return path, nil
}

func DeleteAuthToken() (bool, string, error) {
	path, err := ConfigPath()
	if err != nil {
		return false, "", err
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, path, nil
		}
		return false, path, fmt.Errorf("failed to delete auth config: %w", err)
	}

	return true, path, nil
}

func ResolveToken() (string, string, error) {
	if token := os.Getenv(TokenEnvVar); token != "" {
		return token, "environment", nil
	}

	cfg, err := LoadAuthConfig()
	if err != nil {
		return "", "", err
	}
	if cfg != nil && cfg.APIToken != "" {
		return cfg.APIToken, "config", nil
	}

	return "", "", nil
}

func MaskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "********"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
