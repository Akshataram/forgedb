# ForgeDB

Building a fast, crash-safe, embeddable key-value database from scratch in **pure Go** (no CGO).

Inspired by:  
- https://build-your-own.org/database (free Part I)  
- HaloDB design principles

**Goal**: Single-file B+Tree KV store with WAL, memtable, bloom filters, partitioning, crash recovery, and high test coverage — all in ~30 focused days.

**Current status**: Early foundation (page manager + basic node headers + persistence tests)

## Tech Stack (so far)
- Go 1.21+
- Only stdlib + minimal external deps later

## Getting Started

```bash
git clone https://github.com/Akshataram/forgedb.git
cd forgedb
go mod tidy
go run cmd/forgedb/main.go mydb.db
