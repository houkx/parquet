package rle_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/parsyl/parquet/internal/rle"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	width int32
	name  string
	in    []int64
	out   []byte
	err   error
}

func TestRLE(t *testing.T) {
	testCases := []testCase{
		{
			name:  "rle only",
			width: 3,
			in:    append(repeat(4, 100), repeat(5, 100)...),
		},
		{
			name:  "repeated zeros",
			width: 0,
			in:    repeat(0, 10),
		},
		{
			name:  "odd number of repeated zeros",
			width: 1,
			in:    repeat(0, 17),
		},
		{
			name:  "odd number of repeated ones",
			width: 1,
			in:    repeat(1, 17),
		},
		{
			name:  "bitpacking only",
			width: 3,
			in:    mod(3, 100),
		},
		{
			name:  "more bitpacking only",
			width: 3,
			in:    mod(3, 103),
		},
		{
			name:  "single value",
			width: 1,
			in:    []int64{1},
		},
		{
			name:  "odd number of non-repeated values",
			width: 1,
			in:    []int64{1, 0, 1, 1, 0},
		},
		{
			name:  "width 2",
			width: 2,
			in:    []int64{1, 2, 3},
		},
		{
			name:  "width 4",
			width: 4,
			err:   fmt.Errorf("bitwidth 4 is greater than 3 (highest supported)"),
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%02d-%s", i, tc.name), func(t *testing.T) {
			r, err := rle.New(tc.width, len(tc.in))
			if tc.err != nil {
				assert.Error(t, tc.err, err)
				return
			}

			if !assert.NoError(t, err) {
				return
			}

			for _, x := range tc.in {
				r.Write(x)
			}
			b := r.Bytes()
			vals, _, err := r.Read(bytes.NewReader(b))
			if assert.NoError(t, err, tc.name) {
				assert.Equal(t, tc.in, vals[:len(tc.in)], tc.name)
			}
		})
	}
}

func mod(m, c int) []int64 {
	out := make([]int64, c)
	for i := range out {
		out[i] = int64(i % m)
	}
	return out
}

func modbytes(m, c int) []byte {
	out := make([]byte, c)
	for i := range out {
		out[i] = byte(int64(i % m))
	}
	return out
}

func repeat(v int64, c int) []int64 {
	out := make([]int64, c)
	for i := range out {
		out[i] = v
	}
	return out
}

func readRLEBitPacked(r io.Reader, header, width uint64) ([]int64, error) {
	count := (header >> 1) * 8
	if width == 0 {
		return make([]int64, count), nil
	}

	byteCount := (width * count) / 8
	rawBytes := make([]byte, byteCount)
	if _, err := r.Read(rawBytes); err != nil {
		return nil, err
	}

	current := 0
	data := uint64(rawBytes[current])
	mask := uint64((1 << width) - 1)
	left := uint64(8)
	right := uint64(0)
	out := make([]int64, 0, count)
	total := uint64(len(rawBytes) * 8)
	for total >= width {
		if right >= 8 {
			right -= 8
			left -= 8
			data >>= 8
		} else if left-right >= width {
			out = append(out, int64((data>>right)&mask))
			total -= width
			right += width
		} else if current+1 < len(rawBytes) {
			current++
			data |= uint64(rawBytes[current] << left)
			left += 8
		}
	}
	return out, nil
}