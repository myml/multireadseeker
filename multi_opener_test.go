package multireadseeker

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func init() {
	mrand.Seed(time.Now().Unix())
}

func bytesToMultiOpener(data []byte, size int) []Opener {
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
	var mo []Opener
	for i := 0; i < len(data); i += size {
		mo = append(mo, func(i int) Opener {
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
	return mo
}

func TestMultiOpenerRead(t *testing.T) {
	assert := require.New(t)
	for _, size := range []int{100, 1, 0} {
		a := make([]byte, size)
		_, err := io.ReadFull(rand.Reader, a)
		assert.NoError(err)
		mo := bytesToMultiOpener(a, 0)
		f, err := NewMultiOpener(mo...)
		r, err := f.Open()
		assert.NoError(err)
		b, err := io.ReadAll(r)
		assert.NoError(err)
		assert.Equal(a, b)
		assert.NoError(r.Close())
	}
}

func TestMultiOpenerSeek(t *testing.T) {
	assert := require.New(t)
	a := make([]byte, mrand.Intn(1024)+1)
	_, err := io.ReadFull(rand.Reader, a[:])
	assert.NoError(err)
	rawR := bytes.NewReader(a[:])
	mo := bytesToMultiOpener(a[:], 0)
	f, err := NewMultiOpener(mo...)
	assert.NoError(err)
	r, err := f.Open()
	assert.NoError(err)
	for i := 0; i < 1000; i++ {
		offset := mrand.Intn(len(a))
		whence := mrand.Intn(io.SeekEnd + 1)
		_, err = rawR.Seek(int64(offset), whence)
		assert.NoError(err)
		r.Seek(int64(offset), whence)
		assert.NoError(err)
		rawData, err := io.ReadAll(rawR)
		assert.NoError(err)
		data, err := io.ReadAll(io.LimitReader(r, int64(len(a))))
		assert.NoError(err)
		assert.Equal(rawData, data)
		time.Sleep(time.Microsecond)
	}
	assert.NoError(r.Close())
}

// 压力测试，避免偶现bug
func BenchmarkMultiOpenerRead(b *testing.B) {
	newRawBenchmark := func(countSize int) func(b *testing.B) {
		return func(b *testing.B) {
			assert := require.New(b)
			a := make([]byte, 1024*countSize)
			_, err := io.ReadFull(rand.Reader, a[:])
			assert.NoError(err)
			r := bytes.NewReader(a[:])
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err = io.ReadAll(r)
				assert.NoError(err)
			}
		}
	}
	newMultiBenchmark := func(countSize, chunkSize int) func(b *testing.B) {
		return func(b *testing.B) {
			assert := require.New(b)
			a := make([]byte, countSize*1024)
			_, err := io.ReadFull(rand.Reader, a)
			assert.NoError(err)
			mo := bytesToMultiOpener(a, chunkSize)
			f, err := NewMultiOpener(mo...)
			assert.NoError(err)
			r, err := f.Open()
			assert.NoError(err)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err = io.ReadAll(r)
				assert.NoError(err)
			}
			assert.NoError(r.Close())
		}
	}
	for _, chunk := range [][]int{{1, 1}, {1, 32}, {10, 10}, {10, 32}, {100, 10}, {100, 32}} {
		b.Run(fmt.Sprintf("-raw- count size %dKB", chunk[0]), newRawBenchmark(chunk[0]))
		b.Run(
			fmt.Sprintf("multi count size %dKB chunk size %dB", chunk[0], chunk[1]),
			newMultiBenchmark(chunk[0], chunk[1]),
		)
	}
}
