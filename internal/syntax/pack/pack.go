// Package pack is the compact binary encoding the library snapshot is written
// in: a string table, then a body of varint-packed scalars and string indices.
package pack

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// ErrCorrupt reports a stream that does not decode: a truncated body, an
// index past a table, or a length past the end of the data.
var ErrCorrupt = errors.New("pack: corrupt stream")

// Writer accumulates a stream. Strings are interned into the table as they are
// written, so a stream's bytes depend only on the sequence of writes.
type Writer struct {
	body []byte
	strs *stringTable
}

// stringTable is the interned strings of a stream, shared by the writers of
// its sections.
type stringTable struct {
	ids   map[string]uint64
	table []string
	size  int // total bytes of the table's strings
}

// NewWriter returns an empty Writer.
func NewWriter() *Writer {
	return &Writer{strs: &stringTable{ids: make(map[string]uint64)}}
}

// Section writes what write produces as a length-prefixed section, which a
// Reader can decode apart from the rest; it shares the string table.
func (w *Writer) Section(write func(*Writer)) {
	sub := &Writer{strs: w.strs}
	write(sub)
	w.Len(len(sub.body))
	w.body = append(w.body, sub.body...)
}

// Uint writes an unsigned varint.
func (w *Writer) Uint(v uint64) {
	w.body = binary.AppendUvarint(w.body, v)
}

// Int writes a signed varint (zigzag).
func (w *Writer) Int(v int64) {
	w.body = binary.AppendVarint(w.body, v)
}

// Len writes a slice length or a count.
func (w *Writer) Len(n int) {
	if n < 0 {
		panic("pack: negative length")
	}
	w.Uint(uint64(n))
}

// Bool writes one byte.
func (w *Writer) Bool(b bool) {
	if b {
		w.body = append(w.body, 1)
	} else {
		w.body = append(w.body, 0)
	}
}

// Float writes the IEEE-754 bits of f, little-endian.
func (w *Writer) Float(f float64) {
	w.body = binary.LittleEndian.AppendUint64(w.body, math.Float64bits(f))
}

// String writes s as its string-table index, interning it on first use.
func (w *Writer) String(s string) {
	t := w.strs
	id, ok := t.ids[s]
	if !ok {
		id = uint64(len(t.table))
		t.ids[s] = id
		t.table = append(t.table, s)
		t.size += len(s)
	}
	w.Uint(id)
}

// Bytes returns the stream: the string table (count, each length, the joined
// bytes) followed by the body.
func (w *Writer) Bytes() []byte {
	t := w.strs
	out := make([]byte, 0, len(w.body)+t.size+4*len(t.table)+16)
	out = binary.AppendUvarint(out, uint64(len(t.table)))
	for _, s := range t.table {
		out = binary.AppendUvarint(out, uint64(len(s)))
	}
	for _, s := range t.table {
		out = append(out, s...)
	}
	return append(out, w.body...)
}

// Reader decodes a stream written by Writer. A malformed stream sets a sticky
// error and later reads return zero values, so a caller checks Err at the end.
type Reader struct {
	data  []byte
	pos   int
	table []string
	err   error
}

// NewReader parses the string table of data and positions the reader at the
// body. The table's strings are substrings of one copy of the table's bytes.
func NewReader(data []byte) (*Reader, error) {
	r := &Reader{data: data}
	count := r.Len()
	if r.err != nil {
		return nil, r.err
	}
	lengths := make([]int, count)
	size := 0
	for i := range lengths {
		lengths[i] = r.Len()
		if r.err != nil {
			return nil, r.err
		}
		if lengths[i] > len(data)-r.pos-size {
			r.fail("string table")
			return nil, r.err
		}
		size += lengths[i]
	}
	if size > len(data)-r.pos {
		r.fail("string table")
		return nil, r.err
	}
	all := string(data[r.pos : r.pos+size])
	r.pos += size
	r.table = make([]string, count)
	off := 0
	for i, n := range lengths {
		r.table[i] = all[off : off+n]
		off += n
	}
	return r, nil
}

// Uint reads an unsigned varint.
func (r *Reader) Uint() uint64 {
	if r.err != nil {
		return 0
	}
	if r.pos < len(r.data) && r.data[r.pos] < 0x80 { // the common one-byte value
		v := r.data[r.pos]
		r.pos++
		return uint64(v)
	}
	v, n := binary.Uvarint(r.data[r.pos:])
	if n <= 0 {
		r.fail("varint")
		return 0
	}
	r.pos += n
	return v
}

// Int reads a signed varint.
func (r *Reader) Int() int64 {
	if r.err != nil {
		return 0
	}
	if r.pos < len(r.data) && r.data[r.pos] < 0x80 {
		v := r.data[r.pos]
		r.pos++
		return int64(v>>1) ^ -int64(v&1) // zig-zag, as binary.Varint
	}
	v, n := binary.Varint(r.data[r.pos:])
	if n <= 0 {
		r.fail("varint")
		return 0
	}
	r.pos += n
	return v
}

// Len reads a slice length or count. A length beyond the bytes left is
// corrupt, since every element takes at least one byte.
func (r *Reader) Len() int {
	v := r.Uint()
	if r.err == nil && v > uint64(len(r.data[r.pos:])) {
		r.fail("length")
		return 0
	}
	return int(v) // #nosec G115 -- at most the bytes left, which is an int
}

// Bool reads one byte.
func (r *Reader) Bool() bool {
	if r.err != nil {
		return false
	}
	if r.pos >= len(r.data) {
		r.fail("bool")
		return false
	}
	b := r.data[r.pos]
	r.pos++
	return b != 0
}

// Float reads a little-endian IEEE-754 value.
func (r *Reader) Float() float64 {
	if r.err != nil {
		return 0
	}
	if len(r.data)-r.pos < 8 {
		r.fail("float")
		return 0
	}
	v := binary.LittleEndian.Uint64(r.data[r.pos:])
	r.pos += 8
	return math.Float64frombits(v)
}

// String reads a string-table index.
func (r *Reader) String() string {
	id := r.Uint()
	if r.err != nil {
		return ""
	}
	if id >= uint64(len(r.table)) {
		r.fail("string index")
		return ""
	}
	return r.table[id]
}

// Section returns a Reader over the next length-prefixed section, which the
// receiver skips; the two keep separate errors and may be read concurrently.
func (r *Reader) Section() *Reader {
	n := r.Len()
	if r.err != nil {
		return &Reader{err: r.err, table: r.table}
	}
	sub := &Reader{data: r.data[r.pos : r.pos+n], table: r.table}
	r.pos += n
	return sub
}

// Err reports the first malformation met, or nil.
func (r *Reader) Err() error { return r.err }

// Done reports whether the whole stream has been read without error.
func (r *Reader) Done() bool { return r.err == nil && r.pos == len(r.data) }

// Fail records a malformation the caller found in the decoded values, such as
// an index past a table it built.
func (r *Reader) Fail(what string) {
	r.fail(what)
}

func (r *Reader) fail(what string) {
	if r.err == nil {
		r.err = fmt.Errorf("%w: %s at byte %d", ErrCorrupt, what, r.pos)
	}
}
