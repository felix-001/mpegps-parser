// Package bitreader provides basic interfaces to read and traverse
// an io.Reader as a stream of bits, rather than a stream of bytes.
package bitreader

import (
	"errors"
	"io"
)

// Reader1 is the interface that wraps the basic Read1 method
//
// Read1() will return true or false depending on whether or not
// the next bit in the bit stream is set, then advance one bit
// forward in the bit-stream.
//
// Read1() is the equivalent to Peek1() followed by Skip(1)
type Reader interface {
	Read1() (bool, error)
}

type Reader8 interface {
	Reader
	Read8(n uint) (uint8, error)
}

type Reader16 interface {
	Reader8
	Read16(n uint) (uint16, error)
}

// Reader32 is the interface that wraps the basic Read32 method.
//
// Read32 allows for reading multiple bits, where (1 <= n <= 32) as a uint32
// from the bit stream. Then advancing the bit stream by n bits.
//
// Read32(n) is equivalent to Peek32(n) followed by Skip(n)
type Reader32 interface {
	Reader16
	Read32(n uint) (uint32, error)
}

type Reader64 interface {
	Reader32
	Read64(n uint) (uint64, error)
}

// Peeker is the interface that wraps the basic Peek1 method
//
// Peek1 will return true or false depending on whether or not
// the next bit in the bit stream is set; it does not advance
// the stream any bits.
type Peeker interface {
	Peek1() (bool, error)
}

type Peeker8 interface {
	Peeker
	Peek8(n uint) (uint8, error)
}

type Peeker16 interface {
	Peeker8
	Peek16(n uint) (uint16, error)
}

// Peeker32 is the interface that wraps the basic Peek32 method.
//
// Peek32 allows for reading multiple bits, where (1 <= n <= 32) as a uint32
// from the bit stream; it does not advance
// the bit stream any bits.
type Peeker32 interface {
	Peeker16
	Peek32(n uint) (uint32, error)
}

type Peeker64 interface {
	Peeker32
	Peek64(n uint) (uint64, error)
}

// Skipper is the interface that wraps the basic Skip method.
//
// Skip will advance the bit stream by n bits. Note, n is not
// constrained, like PeekX and ReadX methods. You can skip
// any number of bits up to max uint; the reader will continue
// to fill and drain the buffer until complete.
type Skipper interface {
	Skip(n uint) error
}

// Aligner is the interface that allows for byte realignment.
//
// IsAligned() returns true if the bit stream is currently
// aligned to a byte boundary.
//
// Align() will skip the necessary bit to realign the bit
// stream to a byte boundary. It returns the number of bits skipped
// (0 <= n < 8) during realignment.
type Aligner interface {
	IsAligned() bool
	Align() (n uint, err error)
}

type ByteReader interface {
	Read(p []byte) (n int, err error)
	Len() int
	Size() int64
}

type BitReader1 interface {
	//io.Reader
	ByteReader

	Reader
	Peeker
	Skipper
	Aligner
}

type BitReader8 interface {
	//io.Reader
	ByteReader

	Reader8
	Peeker8
	Skipper
	Aligner
}

type BitReader16 interface {
	//io.Reader
	ByteReader

	Reader16
	Peeker16
	Skipper
	Aligner
}

type BitReader32 interface {
	//io.Reader

	ByteReader
	Reader32
	Peeker32
	Skipper
	Aligner
}

type BitReader interface {
	//io.Reader
	ByteReader
	Reader64
	Peeker64
	Skipper
	Aligner
	//Offset() int
}

// NewBitReader returns the default implementation of a BitReader
func NewReader(r ByteReader) BitReader { // BitReader是一个接口, bitreader实现了
	// 这个接口
	return &bitreader{r: r} // 这里面的r是bytes.Reader,标准库中的
}

type bitreader struct {
	r         ByteReader
	buffer    uint64
	remaining uint
	raw       [8]uint8
}

func (br *bitreader) Read(p []byte) (int, error) {
	br.Align()
	count := int((br.remaining + 7) >> 3)
	if count > len(p) {
		count = len(p)
	}
	for i := 0; i < count; i++ {
		val, err := br.Read8(8)
		if err != nil {
			return i, err
		}
		p[i] = val
	}
	n, err := br.r.Read(p[count:])
	return count + n, err
}

func (br *bitreader) Skip(n uint) error {
	return br.skip(n)
}

func (br *bitreader) IsAligned() bool {
	return br.remaining&0x7 == 0
}

func (br *bitreader) Align() (n uint, err error) {
	n = br.remaining & 0x7
	return n, br.skip(n)
}

func (br *bitreader) Read1() (bool, error) {
	val, err := br.Peek1()
	if err != nil {
		return false, checkEOF(err)
	}
	return val, br.skip(1)
}

func (br *bitreader) Read8(n uint) (uint8, error) {
	if n > 8 {
		return 0, errors.New("overflow")
	}

	val, err := br.read(n)
	return uint8(val), checkEOF(err)
}

func (br *bitreader) Read16(n uint) (uint16, error) {
	if n > 16 {
		return 0, errors.New("overflow")
	}
	val, err := br.read(n)
	return uint16(val), err
}

func (br *bitreader) Read32(n uint) (uint32, error) {
	if n > 32 {
		return 0, errors.New("overflow")
	}
	val, err := br.read(n)
	return uint32(val), err
}

func (br *bitreader) Read64(n uint) (uint64, error) {
	if n > 64 {
		return 0, errors.New("overflow")
	}
	return br.read(n)
}

func (br *bitreader) Peek1() (bool, error) {
	val, err := br.peek(1)
	return err == nil && val == 1, err
}

func (br *bitreader) Peek8(n uint) (uint8, error) {
	if n > 8 {
		return 0, errors.New("overflow")
	}
	val, err := br.peek(n)
	return uint8(val), err
}

func (br *bitreader) Peek16(n uint) (uint16, error) {
	if n > 16 {
		return 0, errors.New("overflow")
	}
	val, err := br.peek(n)
	return uint16(val), err
}

func (br *bitreader) Peek32(n uint) (uint32, error) {
	if n > 32 {
		return 0, errors.New("overflow")
	}
	val, err := br.peek(n)
	return uint32(val), err
}

func (br *bitreader) Peek64(n uint) (uint64, error) {
	if n > 64 {
		return 0, errors.New("overflow")
	}
	return br.peek(n)
}

func (br *bitreader) fill() error {
	// 第一次的时候br.remaining是0，这里的total值是8
	total := (64 - br.remaining) >> 3

	// 这里边传入的br.raw[:total]是一个输出参数，是一个slice
	// Read函数内部根据slice的长度输出数据的长度
	n, err := br.r.Read(br.raw[:total]) // r是标准库的byteReader
	if err != nil {
		return err
	}
	// n是读出来个字节数
	ir := br.remaining
	// 这里是把读取出来的8个字节的slice转换成int64
	// br.remaining指示int64的bufer中有多少个bit没有被读出
	for i := 0; i < n; i++ {
		pos := 64 - 8 - (uint(i) << 3) - ir
		// br.buffer的类型是int64, 8个字节
		br.buffer |= uint64(br.raw[i]) << pos
		br.remaining += 8
	}

	return nil
}

func (br *bitreader) read(n uint) (uint64, error) {
	val, err := br.peek(n)
	if err != nil {
		return 0, checkEOF(err)
	}
	return val, br.skip(n)
}

func (br *bitreader) peek(n uint) (uint64, error) {
	if n > 56 && br.remaining&0x7 != 0 {
		return 0, errors.New("offset mismatch, can't fill the buffer with leftover-bytes")
	}

	for br.remaining < n {
		if err := br.fill(); err != nil {
			return 0, err
		}
	}

	dist := 64 - n
	mask := ^uint64(0) << dist
	result := (br.buffer & mask) >> dist
	return result, nil
}

func (br *bitreader) skip(n uint) error {
	for n > 0 {
		len := n
		if len > br.remaining {
			len = br.remaining
		}

		br.buffer <<= len
		br.remaining -= len
		n -= len

		if n > 0 {
			if err := br.fill(); err != nil {
				return checkEOF(err)
			}
		}
	}

	return nil
}

// Len是没有被读的byte数
func (br *bitreader) Len() int {
	// 每次读取8个字节，有可能int64 buffer里面还没有读出来的数据
	return br.r.Len() + int(br.remaining/8)
}

// size是总长度
func (br *bitreader) Size() int64 {
	return br.r.Size()
}

func checkEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}
