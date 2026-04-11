package main

import (
	"log"
)

// Main entry point for Oido PostgreSQL MCP Server.
// This runs as a standalone process using stdio transport for Qwen CLI.
func main() {
	log.Println("Starting Oido PostgreSQL MCP Server v1.0.0...")
	RunMCPServer()
}
