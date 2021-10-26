package bitreader

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
)

var (
	ErrRequestTooLong = errors.New("err request too long")
)

// 按byte读取
type ByteReader interface {
	io.Seeker
	Read(p []byte) (n int, err error)
	ReadAt(b []byte, off int64) (n int, err error)
}

type BitReader struct {
	size int64
	r    ByteReader
	data uint64
	// 剩余多少bit没有被读取
	remain uint
}

func New(r ByteReader, size int64) *BitReader {
	return &BitReader{r: r, size: size}
}

func (br *BitReader) Size() int64 {
	return br.size
}

// 从外部数据源读取8个字节存放到br.data
func (br *BitReader) fill() error {
	data := make([]byte, 8)
	if _, err := br.r.Read(data); err != nil {
		return err
	}
	br.data = binary.BigEndian.Uint64(data)
	br.remain = 64
	return nil
}

func (br *BitReader) read(n uint) uint64 {
	mask := ^uint64(0) << (64 - n)
	result := (br.data & mask) >> (64 - n)
	return result
}

func (br *BitReader) update(n uint) {
	br.data <<= n
	br.remain -= n
}

func (br *BitReader) Peek(n uint) (result uint64, err error) {
	if n > 64 {
		log.Println(ErrRequestTooLong, n)
		return 0, ErrRequestTooLong
	}
	if n > br.remain {
		left := n - br.remain
		// 上次剩余的
		result = br.read(br.remain) << left
		// 新数据读取的
		buf := make([]byte, 8)
		offset, _ := br.Offset()
		if _, err := br.r.ReadAt(buf, offset); err != nil {
			return 0, err
		}
		data := binary.BigEndian.Uint64(buf)
		mask := ^uint64(0) << (64 - left)
		result |= (data & mask) >> (64 - left)
		return
	}
	result = br.read(n)
	return
}

func (br *BitReader) Read(n uint) (result uint64, err error) {
	if n > 64 {
		log.Println(ErrRequestTooLong, n)
		return 0, ErrRequestTooLong
	}
	if n > br.remain {
		left := n - br.remain
		// 上次剩余的
		result = br.read(br.remain) << left
		if err = br.fill(); err != nil {
			return
		}
		// 新数据读取的
		result |= br.read(left)
		br.update(left)
		return
	}
	result = br.read(n)
	br.update(n)
	return
}

func (br *BitReader) Offset() (int64, error) {
	offset, err := br.r.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	return offset - int64(br.remain)/8, nil
}

func (br *BitReader) readFromCache(b []byte, n int) (int, error) {
	val := br.data >> (64 - n)
	for i := 0; i < n; i++ {
		b[i] = byte(val >> ((n - i) * 8))
	}
	br.update(uint(n * 8))
	return n, nil
}

func (br *BitReader) ReadBytes(b []byte) (n int, err error) {
	left := int(br.remain) / 8
	if len(b) <= left {
		return br.readFromCache(b, len(b))
	}
	br.readFromCache(b, left)
	ret, err := br.r.Read(b[left:])
	if err != nil {
		return 0, err
	}
	n = ret + int(br.remain)/8
	br.remain = 0
	return
}

func (br *BitReader) ReadAt(b []byte, off int64) (int, error) {
	return br.r.ReadAt(b, off)
}
