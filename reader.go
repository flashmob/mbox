package mbox

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"time"
)

type decoder struct {
	r     io.Reader
	state readState
	// iN bytes read to input
	iN int
	// iPos position in input
	iPos int
	// input bytes from underlying reader
	input []byte

	err error

	matches     int
	escapeCount int
	hPos        int

	header strings.Builder
}

type readState int

// possible values for state
const (
	// readStateHeaderMagic begin matching the magic "From " string
	readStateHeaderMagic readState = iota
	// readStateHeaderValues continue reading the header values until eol
	readStateHeaderValues
	// readStateStartLine reached the start of a line
	readStateStartLine
	// readStatePutEol place an end of line (eol) marker
	readStatePutEol
	// readStateCopy keep copying until the eol
	readStateCopy
	// readStateMatchFrom match a potential "From "
	readStateMatchFrom
	// readStateOutputFrom output a "From ", can be escaped with ">"
	readStateOutputFrom
	// readStateHeaderMagicEOF same as readStateHeaderMagic but we can return with io.EOF
	readStateHeaderMagicEOF
	// readStateEnd at the end of a boundary, we can return with an io.EOF
	readStateEnd
	// readStateNextRecord entering a new message
	readStateNextRecord
)

const escape = '>'

// InvalidFormat error is returned when the file format is invalid
var InvalidFormat = errors.New("invalid file format")

// InvalidHeader error is returned when Header() date is invalid
var InvalidHeader = errors.New("invalid header")

// NewReader returns an io.Reader, ready to decode mbox streams
func NewReader(r io.Reader) *decoder {
	d := new(decoder)
	d.r = r
	return d
}

// Read implements io.Reader
func (r *decoder) Read(p []byte) (int, error) {
	// n counts how many bytes were placed on p
	var i, n int
	if r.input == nil {
		r.input = make([]byte, len(p))
	}
	if r.iPos == r.iN { // at the end or no input?
		// get some input to process
		r.iN, r.err = r.r.Read(r.input)
		if r.err == io.EOF {
			if r.state < readStateStartLine {
				r.err = InvalidHeader
			} else if r.state != readStateEnd {
				r.err = InvalidFormat
			}
		}
		if r.iN == 0 {
			// nothing to process
			return i, r.err
		}
		r.iPos = 0 // reset
	}
	if r.iN == 0 && r.err == io.EOF {
		// nothing to process
		// (this is an edge case where the reader could be recycled, but no more input remains to be read)
		// TestReadMSingle tests for this case
		return i, io.EOF
	}

	for r.iPos < r.iN && i < len(p) {
		switch r.state {
		case readStateNextRecord:
			// a header indicating a boundary was detected in the previous read
			// we can recycle the reader and continue with reading a new message
			r.header.Reset()
			r.state = readStateHeaderValues
		case readStateHeaderMagic, readStateHeaderMagicEOF:
			// match the "From " magic string
			if r.input[r.iPos] == header[r.matches] {
				r.iPos++
				r.matches++
				if r.matches == len(header) {
					lastState := r.state
					r.state = readStateHeaderValues
					r.matches = 0
					if lastState == readStateHeaderMagicEOF {
						// a boundary was detected , we return with an io.EOF
						// Note that the state is not reset, so the reader can be recycled to continue
						// reading the next record.
						r.state = readStateNextRecord
						return i, io.EOF
					}
				}
				continue
			} else {
				if r.state == readStateHeaderMagicEOF {
					r.state = readStatePutEol
					continue
				}
				return n, InvalidFormat
			}
		case readStatePutEol:
			// an eol was previously matched, it's not eof, so write it out
			if len(p)-i > 0 {
				p[i] = newLine
				i++
				r.state = readStateOutputFrom
			}
		case readStateHeaderValues:
			// scan until eol
			length := r.iN - r.iPos
			if i := bytes.Index(r.input[r.iPos:r.iPos+length], eol); i != -1 {
				r.header.Write(r.input[r.iPos : r.iPos+i])
				r.matches = 0
				r.escapeCount = 0
				r.state = readStateStartLine
				r.iPos += i + 1
				continue
			}
			r.header.Write(r.input[r.iPos : r.iPos+length])
			r.iPos += length
		case readStateStartLine:
			// current pos is after a \n
			// match >+
			// else go to state readStateOutputFrom
			if r.input[r.iPos] == escape {
				r.escapeCount++
			} else if r.escapeCount > 0 && r.input[r.iPos] == header[0] {
				// keep matching "From " in another state
				r.matches++
				r.iPos++
				r.state = readStateMatchFrom
				continue

			} else if r.escapeCount == 0 && r.input[r.iPos] == newLine {
				// // eof state, we can pass io.EOF back to caller in this state
				r.iPos++
				r.state = readStateEnd
				continue
			} else {
				// output
				if r.escapeCount > 0 { // tested by TestRead6
					r.state = readStateOutputFrom
				} else {
					r.state = readStateCopy // copy state
				}
				continue
			}
			r.iPos++
		case readStateMatchFrom:
			// match >+"From " that's been escaped
			// if entire "From " matched, then we can just --escapeCount
			// goto state readStateOutputFrom
			if r.matches == len(header) {
				r.escapeCount-- // strip a single ">". Assuming that r.escapeCount > 9
				r.iPos++
				r.state = readStateOutputFrom
				continue
			} else if r.input[r.iPos] == header[r.matches] {
				r.matches++
			} else {
				if r.escapeCount == 0 && r.matches == 0 {
					r.state = readStateCopy
				} else {
					r.state = readStateOutputFrom
				}
				continue
			}
			r.iPos++
		case readStateOutputFrom:
			// output >+"From" pattern
			// first the escapeCount, then the matches
			// if the next char is not eol then move to copy state, else readStateStartLine
			// if the full pattern didn't match, output only the partial match
			for i < len(p) {
				if r.escapeCount > 0 {
					p[i] = escape
					r.escapeCount--
					i++
					n++
				} else if r.matches > 0 {
					p[i] = header[r.hPos]
					r.hPos++
					r.matches--
					i++
					n++
				} else {
					break
				}
			}
			if r.matches == 0 {
				r.hPos = 0
			}
			r.state = readStateCopy
		case readStateCopy:
			// copy state
			// scan until eol
			remaining := len(p) - i // remaining slots we can read
			length := r.iN - r.iPos // length of input to process
			if length > remaining {
				length = remaining
			}
			// if there's a new line, read until eol, then change state
			if i := bytes.Index(r.input[r.iPos:r.iPos+length], eol); i != -1 {
				length = i + 1
				r.matches = 0
				r.escapeCount = 0
				r.state = readStateStartLine
			}
			copied := copy(p[i:], r.input[r.iPos:r.iPos+length])
			n += copied
			r.iPos += length
			i += length
		case readStateEnd:
			// eof state
			// don't do anything here, it can exit if io.EOF is read
			if r.iPos < r.iN {
				// there's still more input, thus we won't get an EOF
				// but it might be the end if we read in the magic "From "
				r.state = readStateHeaderMagicEOF
			}
		}
	}
	return n, nil
}

// Close closes the stream and resets all state
func (r *decoder) Close() error {
	r.header.Reset()
	r.iN = 0
	r.iPos = 0
	r.state = readStateHeaderMagic
	return nil
}

// Header returns the parsed header values, from and date
// err is returned if the date is invalid (Not time.ANSIC)
func (r *decoder) Header() (err error, from string, date time.Time) {
	if r.header.Len() > 0 {
		s := r.header.String()
		if i := strings.Index(s, " "); i != -1 {
			from = s[:i]
			if len(s)-1 > i+1 {
				date, err = time.Parse(time.ANSIC, s[i+1:])
			}
			if err == nil {
				return
			}
		}
	}
	err = InvalidHeader
	return
}
