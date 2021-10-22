package bitreader

import (
	"encoding/binary"
	"io"
	"log"
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
		log.Printf("befor fill %#v", br.data)
		left := n - br.remain
		// 上次剩余的
		result = br.read(br.remain) << left
		if err = br.fill(); err != nil {
			return
		}
		// 新数据读取的
		log.Printf("after fill %#v", br.data)
		result |= br.read(left)
		log.Println("after fill remain", br.remain)
		br.update(left)
		log.Println("after update remain", br.remain)
		return
	}
	log.Printf("%#v remain:%d", br.data, br.remain)
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

func (br *BitReader) ReadBytes(b []byte) (n int, err error) {
	binary.BigEndian.PutUint64(b, br.data>>(64-br.remain))
	ret, err := br.r.Read(b[br.remain/8:])
	if err != nil {
		return 0, err
	}
	return ret + int(br.remain)/8, nil
}

func (br *BitReader) ReadAt(b []byte, off int64) (int, error) {
	return br.r.ReadAt(b, off)
}
