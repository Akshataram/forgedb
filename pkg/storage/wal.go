package storage

import (
	"encoding/binary"
	"io"
	"os"
)

const (
	OpPut    byte = 1
	OpDelete byte = 2
)

type WAL struct {
	file *os.File
}

// OpenWAL opens or creates a Write-Ahead Log file for crash recovery.
func OpenWAL(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	}
	return &WAL{file: f}, nil
}

// AppendPut securely logs an insertion intent to disk before tree modification.
func (w *WAL) AppendPut(key, value []byte) error {
	return w.appendOper(OpPut, key, value)
}

// AppendDelete securely logs a deletion intent to disk before tree modification.
func (w *WAL) AppendDelete(key []byte) error {
	return w.appendOper(OpDelete, key, nil)
}

func (w *WAL) appendOper(op byte, key, val []byte) error {
	// Format: [Op(1)] [KLen(2)] [VLen(2)] [Key] [Val]
	buf := make([]byte, 1+2+2+len(key)+len(val))
	buf[0] = op
	binary.LittleEndian.PutUint16(buf[1:3], uint16(len(key)))
	binary.LittleEndian.PutUint16(buf[3:5], uint16(len(val)))
	copy(buf[5:5+len(key)], key)
	copy(buf[5+len(key):], val)

	_, err := w.file.Write(buf)
	if err != nil {
		return err
	}
	// Force physical write to disk before returning
	return w.file.Sync()
}

// Replay reads the command log and safely executes pending operations on startup.
func (w *WAL) Replay(onPut func(k, v []byte) error, onDelete func(k []byte) error) error {
	_, err := w.file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	for {
		header := make([]byte, 5)
		_, err := io.ReadFull(w.file, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			// A partial sector write means the machine crashed mid-flush.
			// We gracefully stop replaying here.
			if err == io.ErrUnexpectedEOF {
				break
			}
			return err
		}

		op := header[0]
		klen := binary.LittleEndian.Uint16(header[1:3])
		vlen := binary.LittleEndian.Uint16(header[3:5])

		key := make([]byte, klen)
		if _, err := io.ReadFull(w.file, key); err != nil {
			break
		}

		val := make([]byte, vlen)
		if _, err := io.ReadFull(w.file, val); err != nil {
			break
		}

		if op == OpPut {
			// Trigger DB Put
			if err := onPut(key, val); err != nil {
				return err
			}
		} else if op == OpDelete {
			// Trigger DB Delete
			if err := onDelete(key); err != nil {
				return err
			}
		}
	}
	return nil
}

// Clear truncates the WAL after operations are securely synced to the main database file.
func (w *WAL) Clear() error {
	err := w.file.Truncate(0)
	if err != nil {
		return err
	}
	_, err = w.file.Seek(0, io.SeekStart)
	return err
}

func (w *WAL) Close() error {
	return w.file.Close()
}
