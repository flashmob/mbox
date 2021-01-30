package mbox

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

const test1 = `test`

const test2 = `test2
Tis this amazing`

// not escape From
const test3 = `test3
Tis this amazing
From: null@example.com
`

const test3Expected = `test3
Tis this amazing
From: null@example.com
`

// escape From
const test4 = `test4
Tis this amazing
From null@example.com
to be jolly
`

const test4Expected = `test4
Tis this amazing
>From null@example.com
to be jolly
`

const test5 = `test5
Tis this amazing
>From null@example.com
to be jolly
`

const test5Expected = `test5
Tis this amazing
>>From null@example.com
to be jolly
`

const test6 = `test6
Tis this amazing
>>>>>>>>From null@example.com
to be jolly
`

const test6Expected = `test6
Tis this amazing
>>>>>>>>>From null@example.com
to be jolly
`

// part match (write Fro when close called)
const test7 = `test7
Tis this amazing
Fro
`
const test7Expected = test7

// ends with >From
const test8 = `test8
Tis this amazing
>>>>>>>>>>>>>>>>From `

const test8Expected = `test8
Tis this amazing
>>>>>>>>>>>>>>>>>From `

// ends with >>>
const test9 = `test9
Tis this amazing
>>>>>>>>>>>>>>>>`

// 33 > chars (toCopy > 32)
const test10 = `test8
Tis this amazing
>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>From null@example.com
to be jolly
`

//TestHeader should just ensure that the basic header is outputted
func TestHeader(t *testing.T) {
	b := bytes.Buffer{}
	w := NewWriter(&b)
	err := w.Open("test@example.com", time.Now())
	if err != nil {
		t.Error(err)
	}
	_, err = w.Write([]byte(test1))
	if err != nil {
		t.Error(err)
	}
	result := b.String()
	parts := strings.Split(result, " ")
	if len(parts) < 3 {
		t.Error("Expecting at least 3 parts")
	} else {
		if parts[0] != "From" {
			t.Error("From not found in the header")
		}
		if parts[1] != "test@example.com" {
			t.Error("test@example.com not found in the header")
		}
		// parse the date
		end := strings.Index(result, "\n")
		date := result[len(parts[0])+len(parts[1])+2 : end]
		tm, err := time.Parse(time.ANSIC, date)
		if err != nil {
			t.Error("Invalid date,", tm, err)
		}
	}
	err = w.Close()
	if err != nil {
		t.Error(err)
	}
}

// TestData should put a new line at the end
func TestData(t *testing.T) {
	b := bytes.Buffer{}
	w := NewWriter(&b)
	err := w.Open("test@example.com", time.Now())
	if err != nil {
		t.Error(err)
	}
	_, _ = w.Write([]byte(test2))
	err = w.Close()
	if err != nil {
		t.Error(err)
	}

	result := b.String()
	if result[len(result)-1] != '\n' {
		t.Error("expecting a new line at the end")
	}
}

// TestDataCopy is same as TestData, but using io.CopyBuffer with a pre-allocated buffer
func TestDataCopy(t *testing.T) {
	b := bytes.Buffer{}
	w := NewWriter(&b)
	buf := make([]byte, 8) // can only write 8 bytes at a time

	err := w.Open("test@example.com", time.Now())
	if err != nil {
		t.Error(err)
	}
	// wrap in an anonymous struct so that WriteTo is not called. That way buf will get used
	_, err = io.CopyBuffer(struct{ io.Writer }{w}, struct{ io.Reader }{bytes.NewReader([]byte(test2))}, buf)
	if err != nil {
		t.Error(err)
	}
	err = w.Close()
	if err != nil {
		t.Error(err)
	}
	result := b.String()
	if result[len(result)-1] != '\n' {
		t.Error("expecting a new line at the end")
	}
}

// no escape
func TestDataEscapeNo(t *testing.T) {
	b := bytes.Buffer{}
	w := NewWriter(&b)
	err := w.Open("test@example.com", time.Now())
	if err != nil {
		t.Error(err)
	}
	_, err = w.Write([]byte(test3))
	if err != nil {
		t.Error(err)
	}
	err = w.Close()
	if err != nil {
		t.Error(err)
	}
	result := b.String()
	result = result[strings.Index(result, "\n")+1:] // cut the header off
	if result != test3Expected+"\n" {
		t.Error("did not get test3_expected")
	}
}

// yes escape. put a > at front
func TestDataEscapeYes(t *testing.T) {
	b := bytes.Buffer{}
	w := NewWriter(&b)
	err := w.Open("test@example.com", time.Now())
	if err != nil {
		t.Error(err)
	}
	_, err = w.Write([]byte(test4))
	if err != nil {
		t.Error(err)
	}
	err = w.Close()
	if err != nil {
		t.Error(err)
	}
	result := b.String()
	result = result[strings.Index(result, "\n")+1:] // cut the header off
	if result != test4Expected+"\n" {
		t.Error("did not get test4_expected")
	}
}

// yes escape. put a > at front of >
func TestDataEscapeYesDouble(t *testing.T) {
	b := bytes.Buffer{}
	w := NewWriter(&b)
	err := w.Open("test@example.com", time.Now())
	if err != nil {
		t.Error(err)
	}
	_, err = w.Write([]byte(test5))
	if err != nil {
		t.Error(err)
	}
	err = w.Close()
	if err != nil {
		t.Error(err)
	}
	result := b.String()
	result = result[strings.Index(result, "\n")+1:] // cut the header off
	if result != test5Expected+"\n" {
		t.Error("did not get test5_expected")
	}
}

// yes escape. put a > at front of >>>>>>>>
func TestDataEscapeYesMulti(t *testing.T) {
	b := bytes.Buffer{}
	w := NewWriter(&b)
	err := w.Open("test@example.com", time.Now())
	if err != nil {
		t.Error(err)
	}
	_, err = w.Write([]byte(test6))
	if err != nil {
		t.Error(err)
	}
	err = w.Close()
	if err != nil {
		t.Error(err)
	}
	result := b.String()
	result = result[strings.Index(result, "\n")+1:] // cut the header off
	if result != test6Expected+"\n" {
		t.Error("did not get test6_expected")
	}
}

// part match (write Fro when close called)
func TestDataPrematureEnd(t *testing.T) {
	b := bytes.Buffer{}
	w := NewWriter(&b)
	err := w.Open("test@example.com", time.Now())
	if err != nil {
		t.Error(err)
	}
	_, err = w.Write([]byte(test7))
	if err != nil {
		t.Error(err)
	}
	err = w.Close()
	if err != nil {
		t.Error(err)
	}
	result := b.String()
	result = result[strings.Index(result, "\n")+1:] // cut the header off
	if result != test7Expected+"\n" {
		t.Error("did not get test7_expected")
	}
}

func TestDataPrematureEscaped2(t *testing.T) {
	b := bytes.Buffer{}
	w := NewWriter(&b)
	err := w.Open("test@example.com", time.Now())
	if err != nil {
		t.Error(err)
	}
	_, err = w.Write([]byte(test8))
	if err != nil {
		t.Error(err)
	}
	err = w.Close()
	if err != nil {
		t.Error(err)
	}
	result := b.String()
	result = result[strings.Index(result, "\n")+1:] // cut the header off
	if result != test8Expected+"\n" {
		t.Error("did not get test8_expected")
	}
}

func TestDataPrematureEscaped3(*testing.T) {
	b := bytes.Buffer{}
	w := NewWriter(&b)
	w.Open("test@example.com", time.Now())
	n, err := w.Write([]byte(test9))
	w.Close()
	fmt.Println(b.String(), n, err)
}

// // 33 > chars (toCopy > 32)
func TestDataEEscapeOverflow(*testing.T) {
	b := bytes.Buffer{}
	w := NewWriter(&b)
	w.Open("test@example.com", time.Now())
	n, err := w.Write([]byte(test10))
	w.Close()
	fmt.Println(b.String(), n, err)
}
