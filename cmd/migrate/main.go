// Package main provides the database migration CLI.
package main

import (
	"fmt"
	"os"

	"github.com/pressly/goose/v3"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: migrate <up|down|status>")
		os.Exit(1)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		panic(err)
	}

	switch os.Args[1] {
	case "up":
		if err := goose.Up(nil, "migrations"); err != nil {
			fmt.Printf("migrate up failed: %v\n", err)
			os.Exit(1)
		}
	case "down":
		if err := goose.Down(nil, "migrations"); err != nil {
			fmt.Printf("migrate down failed: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := goose.Status(nil, "migrations"); err != nil {
			fmt.Printf("migration status failed: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("unsupported command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
