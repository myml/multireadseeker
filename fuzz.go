// +build gofuzz

package multireadseeker

import (
	"bytes"
	"io"
	"io/ioutil"
	mrand "math/rand"

	_ "github.com/dvyukov/go-fuzz/go-fuzz-dep"
)

func fuzzBytesToMultiOpener(data []byte, size int) []Opener {
	if len(data) == 0 {
		return []Opener{}
	}
	if size == 0 {
		if len(data) > 1 {
			size = mrand.Intn(len(data)-1) + 1
		} else {
			size = 1
		}
	}
	var os []Opener
	for i := 0; i < len(data); i += size {
		os = append(os, func(i int) Opener {
			return OpenFunc(func() (io.ReadSeekCloser, error) {
				var rsc struct {
					*bytes.Reader
					io.Closer
				}
				end := i + size
				if end > len(data) {
					rsc.Reader = bytes.NewReader(data[i:])
				} else {
					rsc.Reader = bytes.NewReader(data[i : i+size])
				}
				rsc.Closer = io.NopCloser(rsc.Reader)
				return rsc, nil
			})
		}(i))
	}
	return os
}

func fuzzBytesToMultiReader(data []byte, size int) []io.ReadSeekCloser {
	if len(data) == 0 {
		return []io.ReadSeekCloser{}
	}
	if size == 0 {
		if len(data) > 1 {
			size = mrand.Intn(len(data)-1) + 1
		} else {
			size = 1
		}
	}
	var rs []io.ReadSeekCloser
	for i := 0; i < len(data); i += size {
		var rsc struct {
			*bytes.Reader
			io.Closer
		}
		end := i + size
		if end > len(data) {
			rsc.Reader = bytes.NewReader(data[i:])
		} else {
			rsc.Reader = bytes.NewReader(data[i : i+size])
		}
		rsc.Closer = io.NopCloser(rsc.Reader)
		rs = append(rs, rsc)
	}
	return rs
}

func Fuzz(data []byte) int {
	{
		rs := fuzzBytesToMultiReader(data, 0)
		r, err := NewMultiReadSeeker(rs...)
		if err != nil {
			panic(err)
		}
		defer r.Close()
		result, err := ioutil.ReadAll(r)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(data, result) {
			panic(err)
		}
	}
	{
		os := fuzzBytesToMultiOpener(data, 0)
		o, err := NewMultiOpener(os...)
		if err != nil {
			panic(err)
		}
		r, err := o.Open()
		if err != nil {
			panic(err)
		}
		defer r.Close()
		result, err := ioutil.ReadAll(r)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(data, result) {
			panic(err)
		}
	}
	return 1
}
