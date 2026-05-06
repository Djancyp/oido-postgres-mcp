package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
)

// configJSON holds settings parsed from --config flag.
type configJSON struct {
	POSTGRES_HOST             string `json:"POSTGRES_HOST"`
	POSTGRES_PORT             string `json:"POSTGRES_PORT"`
	POSTGRES_DATABASE         string `json:"POSTGRES_DATABASE"`
	POSTGRES_USER             string `json:"POSTGRES_USER"`
	POSTGRES_PASSWORD         string `json:"POSTGRES_PASSWORD"`
	POSTGRES_ALLOW_SELECT     string `json:"POSTGRES_ALLOW_SELECT"`
	POSTGRES_ALLOW_INSERT     string `json:"POSTGRES_ALLOW_INSERT"`
	POSTGRES_ALLOW_UPDATE     string `json:"POSTGRES_ALLOW_UPDATE"`
	POSTGRES_ALLOW_DELETE     string `json:"POSTGRES_ALLOW_DELETE"`
	POSTGRES_ALLOW_CREATE     string `json:"POSTGRES_ALLOW_CREATE"`
	POSTGRES_ALLOW_ALTER      string `json:"POSTGRES_ALLOW_ALTER"`
	POSTGRES_ALLOW_DROP       string `json:"POSTGRES_ALLOW_DROP"`
	POSTGRES_ALLOW_TRUNCATE   string `json:"POSTGRES_ALLOW_TRUNCATE"`
}

// Main entry point for Oido PostgreSQL MCP Server.
func main() {
	configStr := flag.String("config", "", "JSON string with PostgreSQL settings (overrides .env, overridden by env vars)")
	flag.Parse()

	if *configStr != "" {
		var cfg configJSON
		if err := json.Unmarshal([]byte(*configStr), &cfg); err != nil {
			log.Fatalf("Failed to parse --config JSON: %v", err)
		}
		setIfNotEnv := func(key, val string) {
			if val != "" && os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
		setIfNotEnv("POSTGRES_HOST", cfg.POSTGRES_HOST)
		setIfNotEnv("POSTGRES_PORT", cfg.POSTGRES_PORT)
		setIfNotEnv("POSTGRES_DATABASE", cfg.POSTGRES_DATABASE)
		setIfNotEnv("POSTGRES_USER", cfg.POSTGRES_USER)
		setIfNotEnv("POSTGRES_PASSWORD", cfg.POSTGRES_PASSWORD)
		setIfNotEnv("POSTGRES_ALLOW_SELECT", cfg.POSTGRES_ALLOW_SELECT)
		setIfNotEnv("POSTGRES_ALLOW_INSERT", cfg.POSTGRES_ALLOW_INSERT)
		setIfNotEnv("POSTGRES_ALLOW_UPDATE", cfg.POSTGRES_ALLOW_UPDATE)
		setIfNotEnv("POSTGRES_ALLOW_DELETE", cfg.POSTGRES_ALLOW_DELETE)
		setIfNotEnv("POSTGRES_ALLOW_CREATE", cfg.POSTGRES_ALLOW_CREATE)
		setIfNotEnv("POSTGRES_ALLOW_ALTER", cfg.POSTGRES_ALLOW_ALTER)
		setIfNotEnv("POSTGRES_ALLOW_DROP", cfg.POSTGRES_ALLOW_DROP)
		setIfNotEnv("POSTGRES_ALLOW_TRUNCATE", cfg.POSTGRES_ALLOW_TRUNCATE)
	}

	log.Println("Starting Oido PostgreSQL MCP Server v1.0.0...")
	RunMCPServer()
}
