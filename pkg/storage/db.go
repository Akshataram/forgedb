package storage

import (
	"bytes"
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

func (db *ForgeDB) AllocBranch() (PageID, Page, error) {
	id := db.NextPage
	offset := int64(id) * PageSize

	p := NewEmptyBranch()
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

// Recursive insert with split propagation
// insert is the recursive helper for Put
func (db *ForgeDB) insert(pageID PageID, key, value []byte) (PageID, []byte, PageID, error) {
	page, err := db.ReadPage(pageID)
	if err != nil {
		return 0, nil, 0, err
	}

	if page.NodeType() == NodeTypeLeaf {
		if !page.IsFull() {
			page.Insert(key, value)
			db.WritePage(pageID, page)
			return 0, nil, 0, nil // no split occurred
		}

		// Split the leaf
		left, right, middleKey := page.Split()

		rightIDLeaf, _, _ := db.AllocPage()

		// Insert the new key into the correct child
		if bytes.Compare(key, middleKey) < 0 {
			left.Insert(key, value)
		} else {
			right.Insert(key, value)
		}

		db.WritePage(pageID, left)
		db.WritePage(rightIDLeaf, right)

		return pageID, middleKey, rightIDLeaf, nil
	}

	// Internal (branch) node - find which child to go to
	n := page.Nkeys()
	childIdx := uint16(0)
	for childIdx < n {
		k, _ := page.GetKeyValueAt(childIdx)
		if len(k) > 0 && bytes.Compare(key, k) < 0 {
			break
		}
		childIdx++
	}

	var childID PageID
	if childIdx > 0 {
		childID = page.GetChild(childIdx - 1)
	} else {
		childID = page.GetChild(0)
	}

	// Recurse into the child
	_, splitKey, rightID, err := db.insert(childID, key, value)
	if err != nil {
		return 0, nil, 0, err
	}

	if rightID == 0 {
		return 0, nil, 0, nil // no split
	}

	// Insert the split key into this internal node
	if !page.IsFull() {
		var buf bytes.Buffer
		binary.Write(&buf, binary.LittleEndian, uint64(rightID))
		page.Insert(splitKey, buf.Bytes())
		db.WritePage(pageID, page)
		return 0, nil, 0, nil
	}

	// Internal node is full, split it
	left, right, middleKey := page.Split()

	rightIDBranch, _, _ := db.AllocBranch()

	if bytes.Compare(splitKey, middleKey) < 0 {
		var buf bytes.Buffer
		binary.Write(&buf, binary.LittleEndian, uint64(rightID))
		left.Insert(splitKey, buf.Bytes())
	} else {
		var buf bytes.Buffer
		binary.Write(&buf, binary.LittleEndian, uint64(rightID))
		right.Insert(splitKey, buf.Bytes())
	}

	db.WritePage(pageID, left)
	db.WritePage(rightIDBranch, right)

	return pageID, middleKey, rightIDBranch, nil
}

// Put inserts a key-value pair with recursive split support
func (db *ForgeDB) Put(key, value []byte) error {
	leftID, splitKey, rightID, err := db.insert(db.Root, key, value)
	if err != nil {
		return err
	}

	if rightID != 0 {
		// Create new root as branch node
		newRootID, newRoot, err := db.AllocBranch()
		if err != nil {
			return err
		}

		var buf1 bytes.Buffer
		binary.Write(&buf1, binary.LittleEndian, uint64(leftID))
		newRoot.Insert(nil, buf1.Bytes()) // Left child gets an empty key

		var buf2 bytes.Buffer
		binary.Write(&buf2, binary.LittleEndian, uint64(rightID))
		newRoot.Insert(splitKey, buf2.Bytes()) // Right child gets the splitKey

		db.WritePage(newRootID, newRoot)

		db.Root = newRootID
		db.saveMeta()
	}

	return nil
}

func (db *ForgeDB) Get(key []byte) ([]byte, bool) {
	return db.get(db.Root, key)
}

func (db *ForgeDB) get(pageID PageID, key []byte) ([]byte, bool) {
	p, err := db.ReadPage(pageID)
	if err != nil {
		return nil, false
	}

	if p.NodeType() == NodeTypeLeaf {
		return p.Get(key)
	}

	n := p.Nkeys()
	if n == 0 {
		return nil, false
	}

	childIdx := uint16(0)
	for childIdx < n {
		k, _ := p.GetKeyValueAt(childIdx)
		if len(k) > 0 && bytes.Compare(key, k) < 0 {
			break
		}
		childIdx++
	}

	if childIdx > 0 {
		childIdx--
	}
	childID := p.GetChild(childIdx)
	return db.get(childID, key)
}
