package main

import (
	"fmt"
	"os"

	"github.com/Akshataram/forgedb/pkg/storage"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  forgedb <dbfile> put <key> <value>")
		fmt.Println("  forgedb <dbfile> get <key>")
		fmt.Println("  forgedb <dbfile> scan")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	db, err := storage.Open(dbPath)
	if err != nil {
		fmt.Printf("Open failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	cmd := os.Args[2]

	switch cmd {
	case "put":
		if len(os.Args) != 5 {
			fmt.Println("Usage: put <key> <value>")
			os.Exit(1)
		}
		key := []byte(os.Args[3])
		value := []byte(os.Args[4])
		if err := db.Put(key, value); err != nil {
			fmt.Printf("Put failed: %v\n", err)
		} else {
			fmt.Println("OK")
		}

	case "get":
		if len(os.Args) != 4 {
			fmt.Println("Usage: get <key>")
			os.Exit(1)
		}
		key := []byte(os.Args[3])
		val, found := db.Get(key)
		if found {
			fmt.Printf("%s\n", val)
		} else {
			fmt.Println("not found")
		}

	case "scan":
		// TODO: full scan (for now we just show last page content)
		fmt.Println("=== Database Scan ===")
		if db.NextPage > 1 {
			lastID := db.NextPage - 1
			p, err := db.ReadPage(lastID)
			if err != nil {
				fmt.Printf("Read failed: %v\n", err)
			} else {
				fmt.Print(p)
			}
		} else {
			fmt.Println("Database is empty")
		}

	default:
		fmt.Println("Unknown command. Use: put, get, or scan")
		os.Exit(1)
	}
}
