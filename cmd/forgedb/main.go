package main

import (
	"fmt"
	"os"

	"github.com/Akshataram/forgedb/pkg/storage"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cmd/forgedb/main.go <database.db> [alloc]")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	db, err := storage.Open(dbPath)
	if err != nil {
		fmt.Printf("Open failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	fmt.Printf("🚀 ForgeDB opened: %s (next page = %d)\n", dbPath, db.NextPage)

	if len(os.Args) > 2 && os.Args[2] == "alloc" {
		id, page, err := db.AllocPage()
		if err != nil {
			fmt.Printf("Alloc failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Allocated page %d → %s\n", id, page)
	}

	// Test persistence
	if db.NextPage > 1 {
		p, _ := db.ReadPage(1)
		fmt.Printf("Read back page 1 after restart: %s\n", p)
	}
}
