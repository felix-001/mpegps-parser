package bitreader

import (
	"encoding/binary"
	"io"
)

// 按byte读取
type ByteReader interface {
	Read(p []byte) (n int, err error)
}

type BitReader struct {
	size   int64
	r      ByteReader
	seeker io.Seeker
	data   uint64
	// 剩余多少bit没有被读取
	remain uint
}

func New(r ByteReader, seeker io.Seeker, size int64) *BitReader {
	return &BitReader{r: r, size: size, seeker: seeker}
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
	mask := ^uint64(0) << n
	result := (br.data & mask) >> (64 - n)
	return result
}

func (br *BitReader) update(n uint) {
	br.data <<= n
	br.remain -= n
}

func (br *BitReader) Read(n uint) (result uint64, err error) {
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
	offset, err := br.seeker.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	return offset + int64(br.remain)/8, nil
}
