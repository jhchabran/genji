package table

import (
	"bytes"
	"errors"
	"io"
	"sync/atomic"

	"github.com/asdine/genji/field"
	"github.com/asdine/genji/record"
	b "github.com/asdine/genji/table/internal"
)

// Errors.
var (
	ErrRecordNotFound = errors.New("not found")
)

// A Table represents a group of records.
type Table interface {
	Reader
	Writer
}

type Reader interface {
	Iterate(func(rowid []byte, r record.Record) bool) error
	Record(rowid []byte) (record.Record, error)
}

type Writer interface {
	Insert(record.Record) (rowid []byte, err error)
	Delete(rowid []byte) error
}

type Pker interface {
	Pk() ([]byte, error)
}

// RecordBuffer contains a list of records. It implements the Table interface.
type RecordBuffer struct {
	tree    *b.Tree
	counter int64
}

// Insert adds a record to the buffer.
func (rb *RecordBuffer) Insert(r record.Record) (rowid []byte, err error) {
	if rb.tree == nil {
		rb.tree = b.TreeNew(bytes.Compare)
	}

	if pker, ok := r.(Pker); ok {
		rowid, err = pker.Pk()
		if err != nil {
			return nil, err
		}
	} else {
		rowid = field.EncodeInt64(atomic.AddInt64(&rb.counter, 1))
	}

	rb.tree.Set(rowid, r)

	return rowid, nil
}

// InsertFrom copies all the records of t to the buffer.
func (rb *RecordBuffer) InsertFrom(t Reader) error {
	var er error
	erit := t.Iterate(func(rowid []byte, r record.Record) bool {
		_, err := rb.Insert(r)
		if err != nil {
			er = err
			return false
		}
		return true
	})

	if er != nil {
		return er
	}

	return erit
}

func (rb *RecordBuffer) Record(rowid []byte) (record.Record, error) {
	if rb.tree == nil {
		rb.tree = b.TreeNew(bytes.Compare)
	}

	r, ok := rb.tree.Get(rowid)
	if !ok {
		return nil, ErrRecordNotFound
	}

	return r, nil
}

func (rb *RecordBuffer) Set(rowid []byte, r record.Record) error {
	if rb.tree == nil {
		rb.tree = b.TreeNew(bytes.Compare)
	}

	rb.tree.Set(rowid, r)
	return nil
}

func (rb *RecordBuffer) Delete(rowid []byte) error {
	ok := rb.tree.Delete(rowid)
	if !ok {
		return ErrRecordNotFound
	}

	return nil
}

func (rb *RecordBuffer) Iterate(fn func(rowid []byte, r record.Record) bool) error {
	if rb.tree == nil {
		rb.tree = b.TreeNew(bytes.Compare)
	}

	e, err := rb.tree.SeekFirst()
	if err == io.EOF {
		return nil
	}

	for k, r, err := e.Next(); err != io.EOF; k, r, err = e.Next() {
		if !fn(k, r) {
			return nil
		}
	}

	e.Close()
	return nil
}
