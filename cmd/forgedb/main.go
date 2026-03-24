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
		fmt.Println("  forgedb <dbfile> range <start> <end>")
		fmt.Println("  forgedb <dbfile> stats")
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
		results, err := db.Scan(nil, nil)
		if err != nil {
			fmt.Printf("Scan failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("=== Database Scan ===")
		for _, kv := range results {
			fmt.Printf("%q → %q\n", kv.Key, kv.Value)
		}
		fmt.Printf("Total: %d records\n", len(results))

	case "range":
		if len(os.Args) != 5 {
			fmt.Println("Usage: range <start> <end>")
			os.Exit(1)
		}
		start := []byte(os.Args[3])
		end := []byte(os.Args[4])
		results, err := db.Scan(start, end)
		if err != nil {
			fmt.Printf("Range scan failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("=== Range Scan [%q, %q] ===\n", start, end)
		for _, kv := range results {
			fmt.Printf("%q → %q\n", kv.Key, kv.Value)
		}
		fmt.Printf("Total: %d records\n", len(results))

	case "stats":
		stats, err := db.Stats()
		if err != nil {
			fmt.Printf("Stats failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("=== Database Statistics ===")
		fmt.Printf("Database Path: %s\n", dbPath)
		fmt.Printf("Total Pages: %d\n", stats.TotalPages)
		fmt.Printf("Leaf Pages: %d\n", stats.LeafPages)
		fmt.Printf("Branch Pages: %d\n", stats.BranchPages)
		fmt.Printf("Total Keys: %d\n", stats.TotalKeys)
		fmt.Printf("Tree Height: %d\n", stats.Height)
		fmt.Printf("Fill Ratio: %.2f%%\n", stats.FillRatio)
		fmt.Printf("Page Size: %d bytes\n", storage.PageSize)

	default:
		fmt.Println("Unknown command. Use: put, get, scan, range, or stats")
		os.Exit(1)
	}
}
