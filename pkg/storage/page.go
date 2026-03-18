package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"
	_ "unsafe"
)

const (
	PageSize      = 4096
	HeaderSize    = 4 // bytes 0-1: node type, 2-3: nkeys
	PtrTableStart = 8 // after header: 2-byte offsets for each entry
)

const (
	NodeTypeLeaf   uint16 = 1
	NodeTypeBranch uint16 = 2
)

// Page represents one fixed-size disk block
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

// GetPtr returns the byte offset (from start of page) where the i-th KV starts
func (p Page) GetPtr(i uint16) uint16 {
	if i >= p.Nkeys() {
		return 0
	}
	pos := PtrTableStart + i*2
	return binary.LittleEndian.Uint16(p[pos : pos+2])
}

// SetPtr writes the offset for the i-th entry
func (p Page) SetPtr(i uint16, offset uint16) {
	pos := PtrTableStart + i*2
	binary.LittleEndian.PutUint16(p[pos:pos+2], offset)
}

// GetKeyValueAt returns key and value at index i (0-based)
func (p Page) GetKeyValueAt(i uint16) (key, value []byte) {
	if i >= p.Nkeys() {
		return nil, nil
	}
	offset := int(p.GetPtr(i)) // ← safe cast uint16 → int
	klen := binary.LittleEndian.Uint16(p[offset : offset+2])
	vlen := binary.LittleEndian.Uint16(p[offset+2 : offset+4])
	keyStart := offset + 4
	valueStart := keyStart + int(klen)
	return p[keyStart:valueStart], p[valueStart : valueStart+int(vlen)]
}

// AddEntry appends a new key-value pair at the end (no sorting yet)
func (p Page) AddEntry(key, value []byte) bool {
	n := p.Nkeys()

	if n >= 500 { // lower cap to be safe during testing
		return false
	}

	// Find the end of used data area
	nextFree := PageSize

	if n > 0 {
		// Scan for the highest offset (safe but slow for now)
		highest := uint16(0)
		for i := uint16(0); i < n; i++ {
			ptr := p.GetPtr(i)
			if ptr > highest {
				highest = ptr
			}
		}
		pos := int(highest)
		klen := int(binary.LittleEndian.Uint16(p[pos : pos+2]))
		vlen := int(binary.LittleEndian.Uint16(p[pos+2 : pos+4]))
		nextFree = pos + 4 + klen + vlen
	} else {
		// Empty page: start after pointer table (reserve ~2KB for pointers)
		nextFree = PtrTableStart + 1024 // 1024 entries × 2 bytes = 2KB reserved
	}

	needed := 4 + len(key) + len(value)
	if nextFree+needed > PageSize {
		return false
	}

	// Write KV at nextFree
	offset := nextFree
	binary.LittleEndian.PutUint16(p[offset:offset+2], uint16(len(key)))
	binary.LittleEndian.PutUint16(p[offset+2:offset+4], uint16(len(value)))
	copy(p[offset+4:offset+4+len(key)], key)
	copy(p[offset+4+len(key):offset+4+len(key)+len(value)], value)

	// Add pointer
	p.SetPtr(n, uint16(offset))

	// Increment nkeys
	p.SetHeader(p.NodeType(), n+1)

	return true
}

// NewEmptyLeaf creates a fresh leaf page
func NewEmptyLeaf() Page {
	p := make(Page, PageSize)
	p.SetHeader(NodeTypeLeaf, 0)
	return p
}

// Get returns the value for the given key, or nil if not found
func (p Page) Get(key []byte) ([]byte, bool) {
	n := p.Nkeys()
	for i := uint16(0); i < n; i++ {
		k, v := p.GetKeyValueAt(i)
		if bytes.Equal(k, key) {
			return v, true
		}
	}
	return nil, false
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
