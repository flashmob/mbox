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
	state writeState
	// iN bytes read to input
	iN int
	// iPos position in input
	iPos int
	// input bytes from underlying reader
	input []byte

	err error

	matches       int
	stuffingCount int

	header strings.Builder
}

type readState int

// possible values for state
const (
	readStateHeaderMagic writeState = iota
	readStateHeaderValues
	readStateStartLine

	readStateCopyMatched

	readStateCopy
	readStateMatchFrom
	readStateMatchStuffing
	readStateEnd
)

var InvalidFormat = errors.New("invalid file format")
var InvalidHeader = errors.New("invalid header")

func NewReader(r io.Reader) *decoder {
	d := new(decoder)
	d.r = r
	return d
}

func (r *decoder) Read(p []byte) (int, error) {
	// n counts how many bytes were placed on p
	var i, n int
	if r.input == nil {
		// todo, get them from a pool?
		r.input = make([]byte, len(p))
	}

	// todo when at EOF remove \n if only last

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

	for r.iPos < r.iN && i < len(p) {
		switch r.state {
		case readStateHeaderMagic:
			// match the "From " magic string
			if r.input[r.iPos] == header[r.matches] {
				r.iPos++
				r.matches++
				if r.matches == len(header) {
					r.state = readStateHeaderValues
					r.matches = 0
				}
				continue
			} else {
				return n, InvalidFormat
			}

		case readStateHeaderValues:
			// scan until eol
			length := r.iN - r.iPos
			if i := bytes.Index(r.input[r.iPos:r.iPos+length], eol); i != -1 {
				r.header.Write(r.input[r.iPos:r.iPos+i])
				r.matches = 0
				r.stuffingCount = 0
				r.state = readStateStartLine
				r.iPos += length - 1
				continue
			}
			r.header.Write(r.input[r.iPos:r.iPos+length])
			r.iPos += length

		case readStateStartLine:
			// match >+
			// else go to state 903
			if r.input[r.iPos] == stuffing[0] {
				r.stuffingCount++
			} else if r.stuffingCount > 0 && r.input[r.iPos] == header[0] {
				// keep matching "From " in another state
				r.matches++
				r.state = 902
				continue
			} else {
				// output
				if r.stuffingCount > 0 {
					r.state = 903
				} else {
					r.state = 904 // copy state
				}
				continue
			}
			r.iPos++

		case 902:
			// match >+"From " that's been escaped
			// if entire "From " matched, then we can just --stuffingCount
			// goto state 903
			if r.matches == len(header) {
				r.stuffingCount-- // strip a single ">". Assuming that r.stuffingCount > 9
				r.iPos++
				r.state = 903
				continue
			} else if r.input[r.iPos] == header[r.matches-1] {
				r.matches++
			} else {
				r.state = 904
				continue
			}
			r.iPos++

		case 903:
			// output >+"From" pattern
			// first the stuffingCount, then the matches
			// if the next char is not eol then move to copy state, else readStateStartLine
			length := len(header)
			for i < len(p) {
				if r.stuffingCount > 0 {
					p[i] = '>'
					r.stuffingCount--
					i++
					n++
				} else if r.matches > 0 {
					p[i] = header[length - r.matches]
					r.matches--
					i++
					n++
				}
			}
			if r.stuffingCount == 0 && r.matches == 0 {
				r.state = 904
			}

		case 904:
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
				r.stuffingCount = 0
				r.state = readStateStartLine
			}
			copied := copy(p[i:], r.input[r.iPos:r.iPos+length])
			n += copied
			r.iPos += length
			i += length
		}
	}
	// end of the input?
	//if r.iPos == r.iN {
	//	r.iPos = 0 // reset
	//}
	return n, nil
}

func (r *decoder) Close() error {
	return nil
}

func (r *decoder) Header() (err error, from string, date time.Time) {
	if r.header.Len() > 0 {
		s := r.header.String()
		if i := strings.Index(s, " "); i != -1 {
			from = s[:i]
			if len(s)-1 > i+1 {
				date, err = time.Parse(time.ANSIC, s[i+1:])
			}

			return
		}
	}
	err = errors.New("invalid header")
	return
}
