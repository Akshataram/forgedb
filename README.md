# ForgeDB

Building a fast, crash-safe, embeddable key-value database from scratch in **pure Go** (no CGO).

Inspired by the design of high-performance B+Tree data stores and [build-your-own.org's Database Tutorial](https://build-your-own.org/database), ForgeDB aims to be a single-file, highly reliable local data engine.

**Current Status**: Fully functional B+Tree logic with crash-safe durability!

## ✨ Features

- **Pure B+Tree Architecture**: Implements complex tree operations including node splitting, leaf/branch balancing, recursive search, and sequential leaf iteration.
- **Node Underflow & Merging**: Dynamically collapses and merges under-filled branches to reclaim physical disk space and keep tree-depth optimal.
- **Write-Ahead Log (WAL)**: Includes a robust, crash-resilient WAL to securely log insertion/deletion intents prior to tree mutations, recovering seamlessly if the system shuts down unexpectedly.
- **O(1) Leaf Scans**: Leaf nodes form a sequential linked list via `NextPageID`, making unbounded iterations and bounded range queries exceptionally fast without retracing the tree depth.
- **In-Place Fragmentation Compaction**: Memory gaps inside nodes created by deletions are physically defragmented when necessary.

## 🛠 Tech Stack

- **Go 1.25+**
- Zero external dependencies (Pure `stdlib`)
- Native binary file formatting (`binary.LittleEndian`)

## 🚀 Getting Started

```bash
# Clone the repository
git clone https://github.com/Akshataram/forgedb.git
cd forgedb

# Build the executable
go build -o forgedb ./cmd/forgedb/
```

## 📖 Usage

ForgeDB operates through a simple command-line interface. All operations run directly against a `.db` file (with an automatic `.wal` companion file for crash protection).

```bash
# Insert or update a key
./forgedb mydb.db put user_1 "Alice"

# Retrieve a key
./forgedb mydb.db get user_1

# Delete a key
./forgedb mydb.db delete user_1

# Output all sequential records
./forgedb mydb.db scan

# Scan within bounded keys
./forgedb mydb.db range user_1 user_10

# Print database statistics (next page, root id, row tracking)
./forgedb mydb.db stats
```

## 📜 License
This project is open-source. See `LICENSE` for details.
