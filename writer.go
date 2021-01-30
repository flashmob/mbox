// mbox implements an mbox writer and reader, using the mboxrd variation
// The implementations are stream-based and use minimal buffering
//
// --- Improved mbox
// https://en.wikipedia.org/wiki/Mbox
// http://fileformats.archiveteam.org/wiki/Mbox
// Quote "Some "improved" mbox formats solve this by always adding a ">" sign to lines with "From " either at the
// start of a line or with one or more ">" signs between the start of the line and the "From ". Then, on reading,
// exporting, or otherwise handling the messages, one ">" sign is stripped from the beginning of a line that contains
// "From " after one or more ">" signs. Thus, the ">" sign count increases by one on encoding, and decreases by one on
// decoding, and everything works in a perfectly reversible way if all software cooperates and doesn't encode too many
// times without decoding (which would result in a ">"-sign pileup), or decode too many times (which would strip more
// ">" signs than necessary and leave a bare "From " in a message that didn't have one to begin with)."
package mbox

import (
	"bytes"
	"io"
	"strings"
	"time"
)

const header = "From " // size: 5
const headerEscaped = ">From "
const stuffing = ">"
const spSize = 32

var stuffingPool [spSize]byte

func init() {
	// pre-allocate some stuffings, for faster copying
	for i := range stuffingPool {
		stuffingPool[i] = '>'
	}
}

type encoder struct {
	w     io.Writer
	state writeState
	n     int
	// from is the from header
	from          string
	date          string
	pos           int
	stuffingCount int
	matches       int
	sb            strings.Builder
}

type writeState int

// possible values for state
const (
	writeStateHeader writeState = iota
	writeStateStartLine
	writeStateCopy
	writeStateMatchFrom
	writeStateMatchStuffing
)

// newLine is the new line character
const newLine = '\n'

// eol is the end of line byte sequence
var eol = []byte{newLine}

// writeByte writes a single byte to the underlying writer
func (w *encoder) writeByte(b byte) (n int, err error) {
	for {
		n, err = w.w.Write([]byte{b})
		if err != nil || n > 0 {
			break
		}
	}
	return
}

func (w *encoder) Write(p []byte) (int, error) {
	w.n = 0
	var (
		n   int
		n64 int64
		err error
	)
	w.pos = 0
	w.n = 0
	for w.pos < len(p) {
		switch w.state {
		case writeStateHeader:
			// write the header (not writing from p, so w.n is 0)
			_, err = io.Copy(w.w, strings.NewReader(w.sb.String()))
			if err != nil {
				return 0, err
			}
			w.state = writeStateStartLine
		case writeStateStartLine:
			// only in this state if we're
			// on the start of a new line / start of message.
			if p[w.pos] == stuffing[0] {
				// keep counting how many >
				w.stuffingCount = 1
				// we don't write it out yet, but move on to next & let caller know we got it
				w.n++
				w.pos++
				w.state = writeStateMatchStuffing
				continue
			} else if p[w.pos] == header[0] {
				// match "From "
				w.matches = 1
				w.n++ // we don't write it out yet, but move on to next & let caller know we got it
				w.pos++
				w.state = writeStateMatchFrom
				continue
			}
			w.state = writeStateCopy

		case writeStateCopy:
			// copy until the end of the line, or end of the data
			length := len(p) - w.pos
			// if there's a new line, then we need to change state after copying
			if i := bytes.Index(p[w.pos:], eol); i != -1 {
				length = i + 1
				w.state = writeStateStartLine
			}
			n64, err = io.Copy(w.w, bytes.NewReader(p[w.pos:w.pos+length]))
			w.n += int(n64)
			if err != nil {
				return w.n, err
			}
			w.pos += int(n64)

		case writeStateMatchStuffing:
			// count '>' (already matched >)
			if p[w.pos] == stuffing[0] {
				w.stuffingCount++
				w.pos++
				continue
			}
			// write out the stuffing
			for w.stuffingCount > 0 {
				toCopy := w.stuffingCount
				if toCopy > spSize {
					toCopy = spSize
				}
				n64, err = io.Copy(w.w, bytes.NewReader(stuffingPool[0:toCopy]))
				w.stuffingCount -= int(n64)
				w.n += int(n64)
				if err != nil {
					return w.n, err
				}
			}
			if p[w.pos] == byte(newLine) {
				w.state = writeStateStartLine
				continue
			}
			if p[w.pos] == header[0] {
				w.pos++
				// match the start of the header
				w.matches = 1
				w.state = writeStateMatchFrom
				continue
			}
			w.state = writeStateCopy
		case writeStateMatchFrom:
			// match "From "
			// if "From " matched then write ">From "
			if w.matches == len(header) {
				w.matches = 0
				n64, err = io.Copy(w.w, bytes.NewReader([]byte(headerEscaped)))
				if err != nil {
					return w.n, err
				}
				w.state = writeStateCopy
				continue
			}

			if p[w.pos] == header[w.matches] {
				w.matches++
				w.pos++
				w.n++
				continue
			}
			// not matched
			// do not escape, write out partial match + byte matched

			n64, err = io.Copy(w.w, bytes.NewReader([]byte(header[:w.matches])))
			// (don't update w.n)
			if err != nil {
				return w.n, err
			}
			// (dont update w.pos += int(n64) )

			n, err = w.writeByte(p[w.pos])
			w.n += n
			if err != nil {
				return w.n, err
			}
			w.pos++
			w.matches = 0
			w.state = writeStateCopy
		}
	}
	return w.n, nil
}

func NewWriter(w io.Writer) *encoder {
	e := new(encoder)
	e.w = w
	return e
}

func (w *encoder) Open(from string, t time.Time) error {
	w.from = from
	w.date = t.UTC().Format(time.ANSIC)
	w.sb.WriteString(header)
	w.sb.WriteString(w.from)
	w.sb.WriteString(" ")
	w.sb.WriteString(w.date)
	w.sb.WriteString(string(newLine))
	return nil
}

func (w *encoder) Close() error {
	defer func() {
		w.state = 0
		w.matches = 0
		w.stuffingCount = 0
		w.sb.Reset()
	}()
	if w.matches == 5 {
		// edge case
		_, err := io.Copy(w.w, bytes.NewReader([]byte(headerEscaped)))
		if err != nil {
			return err
		}
	} else if w.stuffingCount > 0 {
		// another edge case
		for w.stuffingCount > 0 {
			toCopy := w.stuffingCount
			if toCopy > spSize {
				toCopy = spSize
			}
			n64, err := io.Copy(w.w, bytes.NewReader(stuffingPool[0:toCopy]))
			w.stuffingCount -= int(n64)
			w.n += int(n64)
			if err != nil {
				return err
			}
		}
	}
	_, err := w.writeByte(newLine)
	if closer, ok := w.w.(io.Closer); ok {
		return closer.Close()
	}
	return err
}
