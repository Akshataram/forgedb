package storage

import (
	"encoding/binary"
	"fmt"
	"os"
)

type PageID uint64

type ForgeDB struct {
	Path     string
	File     *os.File
	NextPage PageID
	Root     PageID
}

func Open(path string) (*ForgeDB, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}

	db := &ForgeDB{
		Path:     path,
		File:     f,
		NextPage: 1,
		Root:     1,
	}

	stat, _ := f.Stat()
	if stat.Size() == 0 {
		if err := db.initMeta(); err != nil {
			f.Close()
			return nil, err
		}
		if _, _, err := db.AllocPage(); err != nil {
			f.Close()
			return nil, err
		}
	} else {
		if err := db.loadMeta(); err != nil {
			f.Close()
			return nil, err
		}
	}

	return db, nil
}

func (db *ForgeDB) Close() error {
	return db.File.Close()
}

func (db *ForgeDB) initMeta() error {
	meta := make([]byte, PageSize)
	binary.LittleEndian.PutUint64(meta[8:16], uint64(db.Root))
	binary.LittleEndian.PutUint64(meta[16:24], uint64(db.NextPage))
	_, err := db.File.WriteAt(meta, 0)
	if err != nil {
		return err
	}
	return db.File.Sync()
}

func (db *ForgeDB) loadMeta() error {
	meta := make([]byte, PageSize)
	_, err := db.File.ReadAt(meta, 0)
	if err != nil {
		return err
	}
	db.Root = PageID(binary.LittleEndian.Uint64(meta[8:16]))
	db.NextPage = PageID(binary.LittleEndian.Uint64(meta[16:24]))
	if db.NextPage < 2 {
		db.NextPage = 2
	}
	return nil
}

func (db *ForgeDB) AllocPage() (PageID, Page, error) {
	id := db.NextPage
	offset := int64(id) * PageSize

	p := NewEmptyLeaf()
	_, err := db.File.WriteAt(p, offset)
	if err != nil {
		return 0, nil, err
	}
	if err := db.File.Sync(); err != nil {
		return 0, nil, err
	}

	db.NextPage++
	db.saveMeta()
	return id, p, nil
}

func (db *ForgeDB) saveMeta() error {
	meta := make([]byte, PageSize)
	binary.LittleEndian.PutUint64(meta[8:16], uint64(db.Root))
	binary.LittleEndian.PutUint64(meta[16:24], uint64(db.NextPage))
	_, err := db.File.WriteAt(meta, 0)
	return err
}

func (db *ForgeDB) ReadPage(id PageID) (Page, error) {
	offset := int64(id) * PageSize
	p := make(Page, PageSize)
	_, err := db.File.ReadAt(p, offset)
	if err != nil {
		return nil, fmt.Errorf("read page %d: %w", id, err)
	}
	return p, nil
}

func (db *ForgeDB) WritePage(id PageID, p Page) error {
	offset := int64(id) * PageSize
	_, err := db.File.WriteAt(p, offset)
	if err != nil {
		return fmt.Errorf("write page %d: %w", id, err)
	}
	return db.File.Sync()
}

func (db *ForgeDB) Put(key, value []byte) error {
	rootPage, err := db.ReadPage(db.Root)
	if err != nil {
		return err
	}

	if rootPage.IsFull() {
		left, right, _ := rootPage.Split()

		leftID, _, _ := db.AllocPage()
		rightID, _, _ := db.AllocPage()

		db.WritePage(leftID, left)
		db.WritePage(rightID, right)

		db.Root = leftID
		db.saveMeta()

		return db.Put(key, value)
	}

	if !rootPage.Insert(key, value) {
		return fmt.Errorf("page full")
	}

	return db.WritePage(db.Root, rootPage)
}

func (db *ForgeDB) Get(key []byte) ([]byte, bool) {
	p, err := db.ReadPage(db.Root)
	if err != nil {
		return nil, false
	}
	return p.Get(key)
}
