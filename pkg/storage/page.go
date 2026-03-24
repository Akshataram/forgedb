package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	PageSize      = 4096
	HeaderSize    = 4
	PtrTableStart = 8
	PtrEntrySize  = 2
)

const (
	NodeTypeLeaf   uint16 = 1
	NodeTypeBranch uint16 = 2
)

type Page []byte

func (p Page) NodeType() uint16 {
	return binary.LittleEndian.Uint16(p[0:2])
}

func (p Page) Nkeys() uint16 {
	return binary.LittleEndian.Uint16(p[2:4])
}

func (p Page) SetHeader(typ uint16, nkeys uint16) {
	binary.LittleEndian.PutUint16(p[0:2], typ)
	binary.LittleEndian.PutUint16(p[2:4], nkeys)
}

func (p Page) GetPtr(i uint16) uint16 {
	if i >= p.Nkeys() {
		return 0
	}
	pos := PtrTableStart + i*PtrEntrySize
	return binary.LittleEndian.Uint16(p[pos : pos+PtrEntrySize])
}

func (p Page) SetPtr(i uint16, offset uint16) {
	pos := PtrTableStart + i*PtrEntrySize
	binary.LittleEndian.PutUint16(p[pos:pos+PtrEntrySize], offset)
}

func (p Page) GetKeyValueAt(i uint16) (key, value []byte) {
	if i >= p.Nkeys() {
		return nil, nil
	}
	offset := int(p.GetPtr(i))
	klen := binary.LittleEndian.Uint16(p[offset : offset+2])
	vlen := binary.LittleEndian.Uint16(p[offset+2 : offset+4])
	keyStart := offset + 4
	valueStart := keyStart + int(klen)
	return p[keyStart:valueStart], p[valueStart : valueStart+int(vlen)]
}

// Insert inserts key-value in sorted order
func (p Page) Insert(key, value []byte) bool {
	n := p.Nkeys()

	if n >= 500 {
		return false
	}

	// Find correct position to insert (keep keys sorted)
	insertPos := uint16(0)
	for i := uint16(0); i < n; i++ {
		k, _ := p.GetKeyValueAt(i)
		cmp := bytes.Compare(key, k)
		if cmp < 0 {
			break
		}
		if cmp == 0 {
			p.updateValueAt(i, value)
			return true
		}
		insertPos++
	}

	// Find free space from the end
	nextFree := PageSize
	if n > 0 {
		highest := uint16(0)
		for i := uint16(0); i < n; i++ {
			if ptr := p.GetPtr(i); ptr > highest {
				highest = ptr
			}
		}
		pos := int(highest)
		klen := int(binary.LittleEndian.Uint16(p[pos:pos+2]))
		vlen := int(binary.LittleEndian.Uint16(p[pos+2:pos+4]))
		nextFree = pos + 4 + klen + vlen
	} else {
		nextFree = PtrTableStart + 1024
	}

	needed := 4 + len(key) + len(value)
	if nextFree+needed > PageSize {
		return false
	}

	// Use int for offset to avoid type errors
	offset := nextFree
	binary.LittleEndian.PutUint16(p[offset:offset+2], uint16(len(key)))
	binary.LittleEndian.PutUint16(p[offset+2:offset+4], uint16(len(value)))
	copy(p[offset+4:offset+4+len(key)], key)
	copy(p[offset+4+len(key):offset+4+len(key)+len(value)], value)

	// Shift pointers
	for i := n; i > insertPos; i-- {
		p.SetPtr(i, p.GetPtr(i-1))
	}

	p.SetPtr(insertPos, uint16(offset))
	p.SetHeader(p.NodeType(), n+1)

	return true
}

func (p Page) updateValueAt(idx uint16, value []byte) {
	off := int(p.GetPtr(idx))
	vlenPos := off + 2
	binary.LittleEndian.PutUint16(p[vlenPos:vlenPos+2], uint16(len(value)))
	copy(p[vlenPos+2:vlenPos+2+len(value)], value)
}

// Get with binary search (fast because keys are sorted)
func (p Page) Get(key []byte) ([]byte, bool) {
	n := p.Nkeys()
	if n == 0 {
		return nil, false
	}

	low, high := uint16(0), n-1
	for low <= high {
		mid := (low + high) / 2
		k, v := p.GetKeyValueAt(mid)
		cmp := bytes.Compare(key, k)
		if cmp == 0 {
			return v, true
		} else if cmp < 0 {
			high = mid - 1
		} else {
			low = mid + 1
		}
	}
	return nil, false
}

func NewEmptyLeaf() Page {
	p := make(Page, PageSize)
	p.SetHeader(NodeTypeLeaf, 0)
	return p
}

func (p Page) String() string {
	n := p.Nkeys()
	s := fmt.Sprintf("Page[type=%d, nkeys=%d]\n", p.NodeType(), n)
	for i := uint16(0); i < n; i++ {
		k, v := p.GetKeyValueAt(i)
		s += fmt.Sprintf("  %d: %q → %q\n", i, k, v)
	}
	return s
}
