package mbox

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

const readTest1 = `From `

// should error (no \n at the end)
const readTest2 = `From test@example.com Wed Jan 27 02:32:22 2021
>>>>From this should be unescaped`

// should be OK (not reading in the last \n
const readTest3 = `From test@example.com Wed Jan 27 02:32:22 2021
>>>>From this should be unescaped
12345678

`

// should stop reading when FROM is found
const readTest4 = `From test@example.com Wed Jan 27 02:32:22 2021
>>>>From this should be unescaped
12345678

From test@example.com Wed Jan 27 02:32:22 2021
>>Frosty morning

`

func TestReadMagic(t *testing.T) {
	buf := make([]byte, 8)
	var b bytes.Buffer
	r := NewReader(bytes.NewReader([]byte(readTest1)))
	_, err := io.CopyBuffer(struct{ io.Writer }{&b}, struct{ io.Reader }{r}, buf)
	if err != InvalidHeader {
		t.Error("InvalidHeader expected")
	}
	err = r.Close()
	if err != nil {
		t.Error(err)
	}
}

func TestReadHeader(t *testing.T) {
	buf := make([]byte, 8)
	var b bytes.Buffer
	r := NewReader(bytes.NewReader([]byte(readTest2)))
	_, err := io.CopyBuffer(struct{ io.Writer }{&b}, struct{ io.Reader }{r}, buf)

	if err != InvalidFormat {
		t.Error(err)
	}

	err, from, time := r.Header()
	if time.Unix() != 1611714742 {
		t.Error("invalid date")
	}
	if from != "test@example.com" {
		t.Error("expecting test@example.com in header")
	}
	if err != nil {
		t.Error(err)
	}
	err = r.Close()
	if err != nil {
		t.Error(err)
	}
	result := b.String()
	fmt.Print(result)
}


func TestReadLastLine(t *testing.T) {
	buf := make([]byte, 8)
	var b bytes.Buffer
	r := NewReader(bytes.NewReader([]byte(readTest3)))
	_, err := io.CopyBuffer(struct{ io.Writer }{&b}, struct{ io.Reader }{r}, buf)

	if err != InvalidFormat {
		t.Error(err)
	}

	err, from, time := r.Header()
	if time.Unix() != 1611714742 {
		t.Error("invalid date")
	}
	if from != "test@example.com" {
		t.Error("expecting test@example.com in header")
	}
	if err != nil {
		t.Error(err)
	}
	err = r.Close()
	if err != nil {
		t.Error(err)
	}
	result := b.String()
	fmt.Print(result)
}
