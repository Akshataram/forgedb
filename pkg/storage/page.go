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

func (p Page) NodeType() uint16 { return binary.LittleEndian.Uint16(p[0:2]) }
func (p Page) Nkeys() uint16    { return binary.LittleEndian.Uint16(p[2:4]) }

func (p Page) SetHeader(typ uint16, nkeys uint16) {
	binary.LittleEndian.PutUint16(p[0:2], typ)
	binary.LittleEndian.PutUint16(p[2:4], nkeys)
}

func (p Page) GetPtr(i uint16) uint16 {
	pos := PtrTableStart + i*PtrEntrySize
	return binary.LittleEndian.Uint16(p[pos : pos+PtrEntrySize])
}

func (p Page) SetPtr(i uint16, offset uint16) {
	pos := PtrTableStart + i*PtrEntrySize
	binary.LittleEndian.PutUint16(p[pos:pos+PtrEntrySize], offset)
}

func (p Page) GetKeyValueAt(i uint16) (key, value []byte) {
	offset := int(p.GetPtr(i))
	klen := binary.LittleEndian.Uint16(p[offset : offset+2])
	vlen := binary.LittleEndian.Uint16(p[offset+2 : offset+4])
	return p[offset+4 : offset+4+int(klen)], p[offset+4+int(klen) : offset+4+int(klen)+int(vlen)]
}

func (p Page) IsFull() bool {
	return p.Nkeys() >= 400 // conservative limit for now
}

// Insert keeps keys sorted
func (p Page) Insert(key, value []byte) bool {
	if p.IsFull() {
		return false
	}

	n := p.Nkeys()
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

	offset := nextFree
	binary.LittleEndian.PutUint16(p[offset:offset+2], uint16(len(key)))
	binary.LittleEndian.PutUint16(p[offset+2:offset+4], uint16(len(value)))
	copy(p[offset+4:offset+4+len(key)], key)
	copy(p[offset+4+len(key):offset+4+len(key)+len(value)], value)

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
	copy(p[vlenPos+2 : vlenPos+2+len(value)], value)
}

func (p Page) Get(key []byte) ([]byte, bool) {
	n := p.Nkeys()
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

// Split splits the leaf into two pages and returns the middle key to promote
func (p Page) Split() (left, right Page, middleKey []byte) {
	n := p.Nkeys()
	splitPoint := n / 2

	left = NewEmptyLeaf()
	right = NewEmptyLeaf()

	// Copy first half to left
	for i := uint16(0); i < splitPoint; i++ {
		k, v := p.GetKeyValueAt(i)
		left.Insert(k, v)
	}

	// Copy second half to right
	for i := splitPoint; i < n; i++ {
		k, v := p.GetKeyValueAt(i)
		right.Insert(k, v)
	}

	// Middle key to promote (first key of right page)
	middleKey, _ = right.GetKeyValueAt(0)

	return left, right, middleKey
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
