
# mbox library for Go (Golang)


This library implements the popular `mbox` format for storing email messages.

it implements the `io.Reader` and `io.Writer`, as well as `io.Closer` ready to plug in to your project.

The library has been written with streaming in mind. (i.e. data is read/written in chunks).
The writer does not use any internal buffers (other than the slice provided) and keeps allocations to a minimum! 
Both the reader and writer work on chunks of any size, especially handy if you use `io.CopyBuffer` or use in TCP streams and so on.

Unit tests coverage at > 90%, and include some edge cases.

## Did you know?


`mbox` has many variants! This library implements the common `mboxrd` variant, popularized by `qmail`.

For a description of the format, see http://qmail.omnis.ch/man/man5/mbox.html

## Using

`import "github.com/flashmob/mbox"`

### As a Writer

Example: read a message `message.eml`, encode and append to the `mbox` file.


```go

// buf is our working buffer, 
// in this example 4096 bytes is the usual block size of a SSD
buf := make([]byte, 4096)
// input
fin, err := os.Open("./message.eml")
// output (mbox files are append-only)
fout, err := os.OpenFile("./mbox", os.O_APPEND|os.O_WRONLY, 0640 )
mbox := mbox.NewWriter(fout)
err = mbox.Open("test@example.com", time.Now())
if err != nil {
   return err
}
// do the encoding
_, err = io.CopyBuffer(fout, fin, buf)
if err != nil {
   return err
}
err = mbox.Close()
if err != nil {
  return err
}

fin.Close()
fout.Close()

```


### As a Reader

```go 

// buf is our working buffer, 
// in this example 4096 bytes is the usual block size of a SSD
buf := make([]byte, 4096)
// output
fout, err := os.OpenFile("./message.eml", os.O_WRONLY, 0600)
// input (just read the first message)
fin, err := os.Open("./mbox")
mbox := mbox.NewReader(fin)
// do the decoding
_, err = io.CopyBuffer(fout, fin, buf)
if err != nil {
    return err
}
// Note: 
// we could read another message here by using io.CopyBuffer with a new destination
// the mbox reader (fin) will automatically read in the next message sequentially
err = mbox.Close()
if err != nil {
    return err
}
fin.Close()
fout.Close()

```


