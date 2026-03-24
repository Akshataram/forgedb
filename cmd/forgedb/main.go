package main

import (
	"fmt"
	"os"

	"github.com/Akshataram/forgedb/pkg/storage"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  go run cmd/forgedb/main.go <dbfile> put <key> <value>")
		fmt.Println("  go run cmd/forgedb/main.go <dbfile> get <key>")
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
			fmt.Println("put <key> <value>")
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
			fmt.Println("get <key>")
			os.Exit(1)
		}
		key := []byte(os.Args[3])
		val, found := db.Get(key)
		if found {
			fmt.Printf("%s\n", val)
		} else {
			fmt.Println("not found")
		}

	default:
		fmt.Println("Unknown command: use put or get")
	}
}
