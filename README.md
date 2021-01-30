
# mbox library for Go (Golang)


This library implements the popular `mbox` format for storing email messages.

it implements the `io.Reader` and `io.Writer`, as well as `io.Closer` ready to plug in to your project.

The library has been written with streaming in mind. (i.e. data is read/written in chunks).
It does not use any internal buffers (other than the slice provided) and keeps allocations to a minimum! 
It can work on chunks of any size, especially handy if you use `io.CopyBuffer`.

Care has been taken to deal with all errors properly. Unit test include some edge cases.

## Did you know?


`mbox` has many variants! This library implements the common `mboxrd` variant, popularized by `qmail`.

For a description of the format, see http://qmail.omnis.ch/man/man5/mbox.html

## Using

### As a Writer

### As a Reader


