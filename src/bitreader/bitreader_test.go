package bitreader

import (
	"bytes"
	"testing"
)

func testRead(t *testing.T, br *BitReader, n uint, expect uint64) {
	actual, err := br.Read(n)
	if err != nil {
		t.Fatal(err)
		return
	}
	if actual != expect {
		t.Fatalf("Expected %d, got %d", expect, actual)
		return
	}
}

func testBasic(t *testing.T) {
	data := []byte{0x49, 0x8b, 0xad, 0x83, 0xa6, 0xa4, 0xe1, 0xb3, 0x85, 0xb, 0x19}
	br := New(bytes.NewReader(data))

	testRead(t, br, 5, 9)
	// 跨1个字节
	testRead(t, br, 4, 3)
	// 跨两个字节
	testRead(t, br, 23, 0xbad83)
	// 读3个字节
	testRead(t, br, 24, 0xa6a4e1)
	// 读一个bit
	testRead(t, br, 1, 1)
	// 读一个bit
	testRead(t, br, 1, 0)
	// 读三个bit, 读取的bit都在last
	testRead(t, br, 3, 6)
	// 需要读取两个byte
	testRead(t, br, 8, 112)
}

func TestBitReader(t *testing.T) {
	testBasic(t)
}
