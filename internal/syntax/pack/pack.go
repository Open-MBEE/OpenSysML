// Package pack is the compact binary encoding the library snapshot is written in: a string
// table, then varint-packed scalars and string indices, one stream per kind so gzip sees like values together.
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

// The streams of a body. A longer varint is split, its low seven bits in the kind's first
// stream and the rest in its tail stream, so the varied low bytes do not break up the high ones.
const (
	streamLen = iota
	streamBool
	streamFloat
	streamUint
	streamUintTail
	streamInt
	streamIntTail
	streamString
	streamStringTail
	numStreams
)

// Writer accumulates a stream. Strings are interned into the table as they are
// written, so a stream's bytes depend only on the sequence of writes.
type Writer struct {
	streams [numStreams][]byte
	strs    *stringTable
	// discard drops every write: see NewDiscard.
	discard bool
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

// NewDiscard returns a Writer that keeps nothing it is given, for a caller that
// runs an encoder only to learn what it visits.
func NewDiscard() *Writer {
	return &Writer{strs: &stringTable{}, discard: true}
}

// Section writes what write produces as a section, which a Reader can decode apart from the
// rest; it shares the string table and is led by the lengths of its streams.
func (w *Writer) Section(write func(*Writer)) {
	if w.discard {
		write(w)
		return
	}
	sub := &Writer{strs: w.strs}
	write(sub)
	for i := range sub.streams {
		w.Len(len(sub.streams[i]))
	}
	for i := range sub.streams {
		w.streams[i] = append(w.streams[i], sub.streams[i]...)
	}
}

// split writes v to a first and a tail stream.
func (w *Writer) split(first int, v uint64) {
	if w.discard {
		return
	}
	if v < 0x80 {
		w.streams[first] = append(w.streams[first], byte(v))
		return
	}
	w.streams[first] = append(w.streams[first], byte(v&0x7f)|0x80)
	w.streams[first+1] = binary.AppendUvarint(w.streams[first+1], v>>7)
}

// Uint writes an unsigned varint.
func (w *Writer) Uint(v uint64) {
	w.split(streamUint, v)
}

// Int writes a signed varint (zigzag).
func (w *Writer) Int(v int64) {
	w.split(streamInt, uint64(v<<1)^uint64(v>>63)) // #nosec G115 -- zig-zag reinterprets the bits
}

// Len writes a slice length or a count.
func (w *Writer) Len(n int) {
	if n < 0 {
		panic("pack: negative length")
	}
	if w.discard {
		return
	}
	w.streams[streamLen] = binary.AppendUvarint(w.streams[streamLen], uint64(n))
}

// Bool writes one byte.
func (w *Writer) Bool(b bool) {
	if w.discard {
		return
	}
	var v byte
	if b {
		v = 1
	}
	w.streams[streamBool] = append(w.streams[streamBool], v)
}

// Float writes the IEEE-754 bits of f, little-endian.
func (w *Writer) Float(f float64) {
	if w.discard {
		return
	}
	w.streams[streamFloat] = binary.LittleEndian.AppendUint64(w.streams[streamFloat], math.Float64bits(f))
}

// String writes s as its string-table index, interning it on first use.
func (w *Writer) String(s string) {
	if w.discard {
		return
	}
	t := w.strs
	id, ok := t.ids[s]
	if !ok {
		id = uint64(len(t.table))
		t.ids[s] = id
		t.table = append(t.table, s)
		t.size += len(s)
	}
	w.split(streamString, id)
}

// Bytes returns the stream: the string table (count, each length, the joined
// bytes) followed by the body (each stream's length, then the streams).
func (w *Writer) Bytes() []byte {
	t := w.strs
	body := 0
	for _, s := range w.streams {
		body += len(s)
	}
	out := make([]byte, 0, body+t.size+4*len(t.table)+16+numStreams*binary.MaxVarintLen64)
	out = binary.AppendUvarint(out, uint64(len(t.table)))
	for _, s := range t.table {
		out = binary.AppendUvarint(out, uint64(len(s)))
	}
	for _, s := range t.table {
		out = append(out, s...)
	}
	for _, s := range w.streams {
		out = binary.AppendUvarint(out, uint64(len(s)))
	}
	for _, s := range w.streams {
		out = append(out, s...)
	}
	return out
}

// Reader decodes a stream written by Writer. A malformed stream sets a sticky
// error and later reads return zero values, so a caller checks Err at the end.
type Reader struct {
	streams [numStreams][]byte // what is left of each stream
	extra   int                // bytes after the last stream, which nothing reads
	table   []string
	err     error
}

// NewReader parses the string table of data and the lengths of the body's
// streams. The table's strings are substrings of one copy of the table's bytes.
func NewReader(data []byte) (*Reader, error) {
	pos := 0
	length := func(what string) (int, error) {
		v, n := binary.Uvarint(data[pos:])
		if n <= 0 || v > uint64(len(data)-pos-n) { // #nosec G115 -- n is at most the bytes after pos
			return 0, fmt.Errorf("%w: %s at byte %d", ErrCorrupt, what, pos)
		}
		pos += n
		return int(v), nil // #nosec G115 -- at most the bytes left, which is an int
	}
	count, err := length("string table")
	if err != nil {
		return nil, err
	}
	lengths := make([]int, count)
	size := 0
	for i := range lengths {
		if lengths[i], err = length("string table"); err != nil {
			return nil, err
		}
		if lengths[i] > len(data)-pos-size {
			return nil, fmt.Errorf("%w: string table at byte %d", ErrCorrupt, pos)
		}
		size += lengths[i]
	}
	if size > len(data)-pos {
		return nil, fmt.Errorf("%w: string table at byte %d", ErrCorrupt, pos)
	}
	all := string(data[pos : pos+size])
	pos += size
	r := &Reader{table: make([]string, count)}
	off := 0
	for i, n := range lengths {
		r.table[i] = all[off : off+n]
		off += n
	}
	var sizes [numStreams]int
	for i := range sizes {
		if sizes[i], err = length("stream length"); err != nil {
			return nil, err
		}
	}
	for i, n := range sizes {
		if n > len(data)-pos {
			return nil, fmt.Errorf("%w: stream length at byte %d", ErrCorrupt, pos)
		}
		r.streams[i] = data[pos : pos+n : pos+n]
		pos += n
	}
	r.extra = len(data) - pos
	return r, nil
}

// split reads a varint from a first and a tail stream.
func (r *Reader) split(first int) uint64 {
	if r.err != nil {
		return 0
	}
	s := r.streams[first]
	if len(s) == 0 {
		r.fail("varint")
		return 0
	}
	b := s[0]
	r.streams[first] = s[1:]
	if b < 0x80 { // the common one-byte value
		return uint64(b)
	}
	high, n := binary.Uvarint(r.streams[first+1])
	if n <= 0 || high>>57 != 0 {
		r.fail("varint")
		return 0
	}
	r.streams[first+1] = r.streams[first+1][n:]
	return high<<7 | uint64(b&0x7f)
}

// Uint reads an unsigned varint.
func (r *Reader) Uint() uint64 {
	return r.split(streamUint)
}

// Int reads a signed varint.
func (r *Reader) Int() int64 {
	v := r.split(streamInt)
	return int64(v>>1) ^ -int64(v&1) // #nosec G115 -- zig-zag, as binary.Varint
}

// left is the bytes not yet read, over every stream and what follows them.
func (r *Reader) left() uint64 {
	n := r.extra
	for _, s := range r.streams {
		n += len(s)
	}
	return uint64(n) // #nosec G115 -- a sum of lengths is not negative
}

// holds reports whether at least n bytes are left, stopping at the first
// stream that settles it.
func (r *Reader) holds(n uint64) bool {
	for _, s := range r.streams {
		if n <= uint64(len(s)) {
			return true
		}
		n -= uint64(len(s))
	}
	return false
}

// Len reads a slice length or count. A length beyond the bytes left is
// corrupt, since every element takes at least one byte.
func (r *Reader) Len() int {
	if r.err != nil {
		return 0
	}
	v, n := binary.Uvarint(r.streams[streamLen])
	if n <= 0 {
		r.fail("varint")
		return 0
	}
	r.streams[streamLen] = r.streams[streamLen][n:]
	if !r.holds(v) {
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
	s := r.streams[streamBool]
	if len(s) == 0 {
		r.fail("bool")
		return false
	}
	r.streams[streamBool] = s[1:]
	return s[0] != 0
}

// Float reads a little-endian IEEE-754 value.
func (r *Reader) Float() float64 {
	if r.err != nil {
		return 0
	}
	s := r.streams[streamFloat]
	if len(s) < 8 {
		r.fail("float")
		return 0
	}
	r.streams[streamFloat] = s[8:]
	return math.Float64frombits(binary.LittleEndian.Uint64(s))
}

// String reads a string-table index.
func (r *Reader) String() string {
	id := r.split(streamString)
	if r.err != nil {
		return ""
	}
	if id >= uint64(len(r.table)) {
		r.fail("string index")
		return ""
	}
	return r.table[id]
}

// Section returns a Reader over the next section, which the receiver skips;
// the two keep separate errors and may be read concurrently.
func (r *Reader) Section() *Reader {
	sub := &Reader{table: r.table}
	var sizes [numStreams]int
	for i := range sizes {
		sizes[i] = r.Len()
	}
	for i, n := range sizes {
		if r.err == nil && n > len(r.streams[i]) {
			r.fail("section length")
		}
	}
	if r.err != nil {
		sub.err = r.err
		return sub
	}
	for i, n := range sizes {
		sub.streams[i] = r.streams[i][:n:n]
		r.streams[i] = r.streams[i][n:]
	}
	return sub
}

// Err reports the first malformation met, or nil.
func (r *Reader) Err() error { return r.err }

// Done reports whether the whole stream has been read without error.
func (r *Reader) Done() bool { return r.err == nil && r.left() == 0 }

// Fail records a malformation the caller found in the decoded values, such as
// an index past a table it built.
func (r *Reader) Fail(what string) {
	r.fail(what)
}

func (r *Reader) fail(what string) {
	if r.err == nil {
		r.err = fmt.Errorf("%w: %s with %d bytes left", ErrCorrupt, what, r.left())
	}
}
