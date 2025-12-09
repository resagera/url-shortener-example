package config

import (
	"flag"
	"os"
	"strings"
)

type Config struct {
	ServerPort string
	DBPath     string
	BaseURL    string
}

// LoadConfig загружает конфиг в порядке приоритета:
// 1. Значения по умолчанию
// 2. Переменные окружения
// 3. Флаги командной строки (наивысший приоритет)
func LoadConfig() *Config {
	// 1. Значения по умолчанию
	cfg := &Config{
		ServerPort: "8384",
		DBPath:     "./data/shortener.db",
		BaseURL:    "http://localhost:8384",
	}

	// 2. Переменные окружения
	if v := os.Getenv("SHORTENER_SERVER_PORT"); v != "" {
		cfg.ServerPort = v
	}
	if v := os.Getenv("SHORTENER_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("SHORTENER_BASE_URL"); v != "" {
		cfg.BaseURL = v
	}

	// 3. Флаги командной строки
	var (
		flagPort    = flag.String("port", "", "Server port (e.g. 8384)")
		flagDBPath  = flag.String("db-path", "", "Path to SQLite database file")
		flagBaseURL = flag.String("base-url", "", "Base URL for generated short links")
	)

	flag.Parse()

	if *flagPort != "" {
		cfg.ServerPort = *flagPort
	}
	if *flagDBPath != "" {
		cfg.DBPath = *flagDBPath
	}
	if *flagBaseURL != "" {
		cfg.BaseURL = *flagBaseURL
	}

	// Приведение порта к формату ":8384"
	if !strings.HasPrefix(cfg.ServerPort, ":") {
		cfg.ServerPort = ":" + cfg.ServerPort
	}

	return cfg
}
