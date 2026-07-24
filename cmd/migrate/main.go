package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	m, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		log.Fatalf("Failed to create migrator: %v", err)
	}
	defer m.Close()

	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("Usage: migrate <up|down|drop|version|goto N>")
	}

	cmd := args[0]
	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate up: %v", err)
		}
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate down: %v", err)
		}
	case "drop":
		if err := m.Drop(); err != nil {
			log.Fatalf("migrate drop: %v", err)
		}
	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("migrate version: %v", err)
		}
		fmt.Printf("Version: %d, Dirty: %t\n", v, dirty)
	case "goto":
		if len(args) < 2 {
			log.Fatal("goto requires a version number")
		}
		n, err := strconv.ParseUint(strings.TrimSpace(args[1]), 10, 64)
		if err != nil {
			log.Fatalf("invalid version: %v", err)
		}
		if err := m.Steps(int(n) - 1); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate goto: %v", err)
		}
	default:
		log.Fatalf("unknown command: %s (use up, down, drop, version, goto)", cmd)
	}
}
