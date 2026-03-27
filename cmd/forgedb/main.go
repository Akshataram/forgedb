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

	case "delete":
		if len(os.Args) != 4 {
			fmt.Println("Usage: delete <key>")
			os.Exit(1)
		}
		key := []byte(os.Args[3])
		if err := db.Delete(key); err != nil {
			fmt.Printf("Delete failed: %v\n", err)
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
		fmt.Println("=== Full Scan ===")
		err := db.RangeScan(nil, nil, func(k, v []byte) bool {
			fmt.Printf("%q → %q\n", k, v)
			return true
		})
		if err != nil && err.Error() != "stopped" {
			fmt.Printf("Scan failed: %v\n", err)
		}

	case "range":
		if len(os.Args) != 5 {
			fmt.Println("Usage: range <start> <end>")
			os.Exit(1)
		}
		start := []byte(os.Args[3])
		end := []byte(os.Args[4])
		fmt.Printf("=== Range Scan [%s to %s] ===\n", start, end)

		err := db.RangeScan(start, end, func(k, v []byte) bool {
			fmt.Printf("%q → %q\n", k, v)
			return true
		})
		if err != nil && err.Error() != "stopped" {
			fmt.Printf("Range scan failed: %v\n", err)
		}

	case "stats":
		fmt.Println("=== Database Stats ===")
		fmt.Printf("Database file: %s\n", dbPath)
		fmt.Printf("Next page ID : %d\n", db.NextPage)
		fmt.Printf("Root page ID : %d\n", db.Root)
		if db.NextPage > 1 {
			lastID := db.NextPage - 1
			p, _ := db.ReadPage(lastID)
			fmt.Printf("Keys in last page: %d\n", p.Nkeys())
		}

	default:
		fmt.Println("Unknown command. Use: put, get, scan, range, stats")
		os.Exit(1)
	}
}
