package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	InputDir           string `json:"input_dir"`
	SqlLogDir          string `json:"sql_log_dir"`
	CsvLogDir          string `json:"csv_log_dir"`
	LogsDir            string `json:"logs_dir"`
	MaxAgents          int    `json:"max_agents"`
	MaxFilesPerAgent   int    `json:"max_files_per_agent"`
	DelayBeforeReadMs  int    `json:"delay_before_read_ms"`
	ApiPort            int    `json:"api_port"`
}

func LoadConfig(filename string) (*Config, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("no se pudo determinar el ejecutable: %w", err)
	}
	baseDir := filepath.Dir(exePath)
	path := filepath.Join(baseDir, filename)

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error al abrir archivo de configuración: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	cfg := &Config{}
	err = decoder.Decode(cfg)
	if err != nil {
		return nil, fmt.Errorf("error al decodificar JSON: %w", err)
	}

	// Resolve absolute paths from binary dir
	if !filepath.IsAbs(cfg.InputDir) {
		cfg.InputDir = filepath.Join(baseDir, cfg.InputDir)
	}
	if !filepath.IsAbs(cfg.SqlLogDir) {
		cfg.SqlLogDir = filepath.Join(baseDir, cfg.SqlLogDir)
	}
	if !filepath.IsAbs(cfg.CsvLogDir) {
		cfg.CsvLogDir = filepath.Join(baseDir, cfg.CsvLogDir)
	}
	if !filepath.IsAbs(cfg.LogsDir) {
		cfg.LogsDir = filepath.Join(baseDir, cfg.LogsDir)
	}

	// Create directories if they don't exist
	dirs := []string{cfg.InputDir, cfg.SqlLogDir, cfg.CsvLogDir, cfg.LogsDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("error creando directorio %s: %w", dir, err)
		}
	}

	return cfg, nil
}
