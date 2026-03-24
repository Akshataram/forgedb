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
func (db *ForgeDB) insert(pageID PageID, key, value []byte) (PageID, []byte, PageID, error) {
	page, err := db.ReadPage(pageID)
	if err != nil {
		return 0, nil, 0, err
	}

	if page.NodeType() == NodeTypeLeaf {
		if !page.IsFull() {
			page.Insert(key, value)
			if err := db.WritePage(pageID, page); err != nil {
				return 0, nil, 0, err
			}
			return 0, nil, 0, nil // no split
		}

		// Split leaf
		left, right, middleKey := page.Split()

		leftID, _, err := db.AllocPage()
		if err != nil {
			return 0, nil, 0, err
		}
		rightID, _, err := db.AllocPage()
		if err != nil {
			return 0, nil, 0, err
		}

		db.WritePage(leftID, left)
		db.WritePage(rightID, right)

		// Insert into appropriate child
		if bytes.Compare(key, middleKey) < 0 {
			left.Insert(key, value)
			db.WritePage(leftID, left)
		} else {
			right.Insert(key, value)
			db.WritePage(rightID, right)
		}

		return leftID, middleKey, rightID, nil
	}

	// Internal node - find child to recurse into
	n := page.Nkeys()
	childIdx := uint16(0)
	for childIdx < n {
		k, _ := page.GetKeyValueAt(childIdx)
		if bytes.Compare(key, k) < 0 {
			break
		}
		childIdx++
	}

	var childID PageID
	if childIdx < n {
		val, _ := page.GetKeyValueAt(childIdx)
		binary.Read(bytes.NewReader(val), binary.LittleEndian, &childID)
	} else if n > 0 {
		val, _ := page.GetKeyValueAt(n - 1)
		binary.Read(bytes.NewReader(val), binary.LittleEndian, &childID)
	}

	_, splitKey, rightID, err := db.insert(PageID(childID), key, value)
	if err != nil {
		return 0, nil, 0, err
	}

	if rightID == 0 {
		return 0, nil, 0, nil // no split
	}

	// Insert split key into current node
	if !page.IsFull() {
		// Convert childID to bytes
		var buf bytes.Buffer
		binary.Write(&buf, binary.LittleEndian, uint64(rightID))
		page.Insert(splitKey, buf.Bytes())
		db.WritePage(pageID, page)
		return 0, nil, 0, nil
	}

	// Split internal node
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint64(rightID))
	page.Insert(splitKey, buf.Bytes())

	left, right, middleKey := page.Split()

	newLeftID, _, err := db.AllocBranch()
	if err != nil {
		return 0, nil, 0, err
	}
	newRightID, _, err := db.AllocBranch()
	if err != nil {
		return 0, nil, 0, err
	}

	db.WritePage(newLeftID, left)
	db.WritePage(newRightID, right)

	return newLeftID, middleKey, newRightID, nil
}

func (db *ForgeDB) Put(key, value []byte) error {
	leftID, splitKey, rightID, err := db.insert(db.Root, key, value)
	if err != nil {
		return err
	}

	if rightID != 0 {
		// Create new root
		newRoot := NewEmptyBranch()
		var buf bytes.Buffer
		binary.Write(&buf, binary.LittleEndian, uint64(leftID))
		newRoot.Insert(splitKey, buf.Bytes())
		
		var buf2 bytes.Buffer
		binary.Write(&buf2, binary.LittleEndian, uint64(rightID))
		newRoot.Insert(splitKey, buf2.Bytes())

		newRootID, _, err := db.AllocBranch()
		if err != nil {
			return err
		}
		db.WritePage(newRootID, newRoot)
		db.Root = newRootID
		db.saveMeta()
	}

	return nil
}

func (db *ForgeDB) Get(key []byte) ([]byte, bool) {
	pageID := db.Root
	for {
		page, err := db.ReadPage(pageID)
		if err != nil {
			return nil, false
		}

		if page.NodeType() == NodeTypeLeaf {
			return page.Get(key)
		}

		// Traverse internal node
		n := page.Nkeys()
		childIdx := uint16(0)
		for childIdx < n {
			k, _ := page.GetKeyValueAt(childIdx)
			if bytes.Compare(key, k) < 0 {
				break
			}
			childIdx++
		}

		var childID PageID
		if childIdx < n {
			val, _ := page.GetKeyValueAt(childIdx)
			binary.Read(bytes.NewReader(val), binary.LittleEndian, &childID)
		} else if n > 0 {
			val, _ := page.GetKeyValueAt(n - 1)
			binary.Read(bytes.NewReader(val), binary.LittleEndian, &childID)
		}
		pageID = childID
	}
}
