package main

import (
	"fmt"
	"os"

	"github.com/Akshataram/forgedb/pkg/storage"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cmd/forgedb/main.go <dbfile> [testkv | get <key>]")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	db, err := storage.Open(dbPath)
	if err != nil {
		fmt.Printf("Open failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	fmt.Printf("ForgeDB opened: %s (next page = %d)\n", dbPath, db.NextPage)

	if len(os.Args) > 2 {
		if os.Args[2] == "testkv" {
			id, page, err := db.AllocPage()
			if err != nil {
				fmt.Printf("Alloc failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Allocated fresh page %d (initial nkeys=%d)\n", id, page.Nkeys())

			added1 := page.AddEntry([]byte("name"), []byte("Akshata"))
			added2 := page.AddEntry([]byte("city"), []byte("Vattalkundu"))
			added3 := page.AddEntry([]byte("fruit"), []byte("mango"))

			fmt.Printf("AddEntry results: name=%v, city=%v, fruit=%v\n", added1, added2, added3)
			fmt.Printf("After adds → nkeys=%d\n", page.Nkeys())

			fmt.Printf("Page before save:\n%s\n", page)

			// Save the page
			offset := int64(id) * storage.PageSize
			_, err = db.File.WriteAt(page, offset)
			if err != nil {
				fmt.Printf("Write failed: %v\n", err)
				os.Exit(1)
			}
			err = db.File.Sync()
			if err != nil {
				fmt.Printf("Sync failed: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Successfully wrote page %d\n", id)
		} else if os.Args[2] == "get" && len(os.Args) > 3 {
			if db.NextPage <= 1 {
				fmt.Println("No data pages yet. Run with testkv first.")
				os.Exit(1)
			}
			lastID := db.NextPage - 1
			p, err := db.ReadPage(lastID)
			if err != nil {
				fmt.Printf("Read failed: %v\n", err)
				os.Exit(1)
			}
			key := []byte(os.Args[3])
			val, found := p.Get(key)
			if found {
				fmt.Printf("GET %q → %q\n", key, val)
			} else {
				fmt.Printf("GET %q → not found\n", key)
			}
		}
	}

	// Always show last page if exists
	if db.NextPage > 1 {
		lastID := db.NextPage - 1
		p, err := db.ReadPage(lastID)
		if err != nil {
			fmt.Printf("Read failed: %v\n", err)
		} else {
			fmt.Printf("Last page (%d) content:\n%s\n", lastID, p)
		}
	}
}
