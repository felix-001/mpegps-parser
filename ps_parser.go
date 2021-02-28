package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mpegps-parser/bitreader"
	"os"
)

const (
	StartCodePS    = 0x000001ba
	StartCodeSYS   = 0x000001bb
	StartCodeMAP   = 0x000001bc
	StartCodeVideo = 0x000001e0
	StartCodeAudio = 0x000001c0
)

var (
	ErrNotFoundStartCode = errors.New("not found the need start code flag")
	ErrFormatPack        = errors.New("not package standard")
	ErrParsePakcet       = errors.New("parse ps packet error")
	ErrNewBiteReader     = errors.New("new bit reader error")
	ErrCheckH264         = errors.New("check h264 error")
)

type FieldInfo struct {
	len  uint
	item string
}

type PsDecoder struct {
	videoStreamType    uint32
	audioStreamType    uint32
	br                 bitreader.BitReader
	psHeader           map[string]uint32
	handlers           map[int]func() error
	psHeaderFields     []FieldInfo
	pktCnt             int
	fileSize           int
	psBuf              *[]byte
	errVideoFrameCnt   int
	totalVideoFrameCnt int
	iFrameCnt          int
	psmCnt             int
	errIFrameCnt       int
}

func (dec *PsDecoder) decodePsPkts() error {
	for dec.getPos() < int64(dec.fileSize) {
		startCode, err := dec.br.Read32(32)
		if err != nil {
			log.Println(err)
			return err
		}
		dec.pktCnt++
		fmt.Println("")
		log.Printf("pkt count: %d pos: %d/%d", dec.pktCnt, dec.getPos(), dec.fileSize)
		handler, ok := dec.handlers[int(startCode)]
		if !ok {
			log.Printf("check startCode error: 0x%x\n", startCode)
			return ErrParsePakcet
		}
		handler()
	}
	return nil
}

func (dec *PsDecoder) decodeSystemHeader() error {
	log.Println("=== ps system header === ")
	br := dec.br
	syslens, err := br.Read32(16)
	log.Printf("\tsystem_header_length:%d", syslens)
	if err != nil {
		return err
	}

	br.Skip(uint(syslens) * 8)
	return nil
}

func (decoder *PsDecoder) getPos() int64 {
	pos := decoder.br.Size() - int64(decoder.br.Len())
	return pos
}

func (decoder *PsDecoder) decodePsmNLoop(programStreamMapLen uint32) error {
	br := decoder.br
	for programStreamMapLen > 0 {
		streamType, err := br.Read32(8)
		log.Printf("\t\tstream type: 0x%x", streamType)
		if err != nil {
			return err
		}
		elementaryStreamID, err := br.Read32(8)
		if err != nil {
			return err
		}
		if elementaryStreamID >= 0xe0 && elementaryStreamID <= 0xef {
			decoder.videoStreamType = streamType
		}
		if elementaryStreamID >= 0xc0 && elementaryStreamID <= 0xdf {
			decoder.audioStreamType = streamType
		}
		log.Printf("\t\tstream id: 0x%x", elementaryStreamID)
		elementaryStreamInfoLength, err := br.Read32(16)
		if err != nil {
			return err
		}
		log.Printf("\t\telementary_stream_info_length: %d", elementaryStreamInfoLength)
		br.Skip(uint(elementaryStreamInfoLength * 8))
		programStreamMapLen -= (4 + elementaryStreamInfoLength)
	}
	return nil
}

func (dec *PsDecoder) decodeProgramStreamMap() error {
	log.Println("=== program stream map ===")
	br := dec.br
	dec.psmCnt++
	psmLen, err := br.Read32(16)
	if err != nil {
		return err
	}
	log.Printf("\tprogram_stream_map_length: %d pos: %d", psmLen, dec.getPos())
	//drop psm version info
	br.Skip(16)
	psmLen -= 2
	programStreamInfoLen, err := br.Read32(16)
	if err != nil {
		return err
	}
	br.Skip(uint(programStreamInfoLen * 8))
	psmLen -= (programStreamInfoLen + 2)
	programStreamMapLen, err := br.Read32(16)
	if err != nil {
		return err
	}
	psmLen -= (2 + programStreamMapLen)
	log.Printf("\tprogram_stream_info_length: %d", programStreamMapLen)

	if err := dec.decodePsmNLoop(programStreamMapLen); err != nil {
		return err
	}

	// crc 32
	if psmLen != 4 {
		log.Printf("psmLen: 0x%x", psmLen)
		return ErrFormatPack
	}
	br.Skip(32)
	return nil
}

func (dec *PsDecoder) decodeH264(data []byte, len uint32, err bool) error {
	log.Printf("\t\th264 len : %d", len)
	if data[4] == 0x67 {
		log.Println("\t\tSPS")
	}
	if data[4] == 0x68 {
		log.Println("\t\tPPS")
	}
	if data[4] == 0x65 {
		log.Println("\t\tIDR")
		if err {
			dec.errIFrameCnt++
		} else {
			dec.iFrameCnt++
		}
	}
	if data[4] == 0x61 {
		log.Println("\t\tP Frame")
	}
	return nil
}

func (dec *PsDecoder) isStartCodeValid(startCode uint32) bool {
	if startCode == StartCodePS ||
		startCode == StartCodeMAP ||
		startCode == StartCodeSYS ||
		startCode == StartCodeVideo ||
		startCode == StartCodeAudio {
		return true
	}
	return false
}

func (dec *PsDecoder) checkH264(h264Len uint32) bool {
	psBuf := *dec.psBuf
	pos := dec.getPos() + int64(h264Len)
	packStartCode := binary.BigEndian.Uint32(psBuf[pos : pos+4])
	if !dec.isStartCodeValid(packStartCode) {
		log.Printf("check start code error: 0x%x pos: %d", packStartCode, dec.getPos())
		return false
	}
	return true
}

func (dec *PsDecoder) GetNextPackPos() (int, bool) {
	pos := int(dec.getPos())
	for pos < dec.fileSize-4 {
		b := (*dec.psBuf)[pos : pos+4]
		packStartCode := binary.BigEndian.Uint32(b)
		if dec.isStartCodeValid((packStartCode)) {
			return pos, false
		}
		pos++
	}
	return 0, true
}

func (dec *PsDecoder) decodePESPacket() error {
	log.Println("=== video ===")
	br := dec.br
	dec.totalVideoFrameCnt++
	payloadLen, err := br.Read32(16)
	if err != nil {
		return err
	}
	log.Printf("\tPES_packet_length: %d", payloadLen)
	br.Skip(16) // 跳过各种flags,比如pts_dts_flags

	payloadLen -= 2
	pesHeaderDataLen, err := br.Read32(8)
	if err != nil {
		return err
	}
	log.Printf("\tpes_header_data_length: %d", pesHeaderDataLen)
	payloadLen--
	br.Skip(uint(pesHeaderDataLen * 8))
	payloadLen -= pesHeaderDataLen
	if !dec.checkH264(payloadLen) {
		dec.errVideoFrameCnt++
		log.Println("check h264 error")
		pos, end := dec.GetNextPackPos()
		if !end {
			log.Printf("% X\n", (*dec.psBuf)[pos:pos+32])
			log.Printf("skip pos: %d", pos)
			skipLen := pos - int(dec.getPos())
			log.Printf("skip len: %d", skipLen)
			skipBuf := make([]byte, skipLen)
			if _, err := io.ReadAtLeast(br, skipBuf, int(skipLen)); err != nil {
				log.Println(err)
				return err
			}
			dec.decodeH264(skipBuf, uint32(skipLen), true)
			return ErrCheckH264
		}
	}
	payloadData := make([]byte, payloadLen)
	if _, err := io.ReadAtLeast(br, payloadData, int(payloadLen)); err != nil {
		return err
	}
	dec.decodeH264(payloadData, payloadLen, false)

	return nil
}

func (decoder *PsDecoder) decodePsHeader() error {
	log.Println("=== pack header ===")
	psHeaderFields := decoder.psHeaderFields
	for _, field := range psHeaderFields {
		val, err := decoder.br.Read32(field.len)
		if err != nil {
			log.Printf("parse %s error", field.item)
			return err
		}
		decoder.psHeader[field.item] = val
	}
	pack_stuffing_length := decoder.psHeader["pack_stuffing_length"]
	decoder.br.Skip(uint(pack_stuffing_length * 8))
	/*
		b, err := json.MarshalIndent(decoder.psHeader, "", "  ")
		if err != nil {
			log.Println("error:", err)
		}
		fmt.Print(string(b) + "\n")
	*/
	return nil
}

func NewPsDecoder(br bitreader.BitReader, psBuf *[]byte, fileSize int) *PsDecoder {
	psDecoder := &PsDecoder{
		br:             br,
		psHeader:       make(map[string]uint32),
		handlers:       make(map[int]func() error),
		psHeaderFields: make([]FieldInfo, 14),
		fileSize:       fileSize,
		psBuf:          psBuf,
	}
	psDecoder.handlers = map[int]func() error{
		StartCodePS:    psDecoder.decodePsHeader,
		StartCodeSYS:   psDecoder.decodeSystemHeader,
		StartCodeMAP:   psDecoder.decodeProgramStreamMap,
		StartCodeVideo: psDecoder.decodePESPacket,
		StartCodeAudio: psDecoder.decodePESPacket,
	}
	psDecoder.psHeaderFields = []FieldInfo{
		{2, "fixed"},
		{3, "system_clock_refrence_base1"},
		{1, "marker_bit1"},
		{15, "system_clock_refrence_base2"},
		{1, "marker_bit2"},
		{15, "system_clock_refrence_base3"},
		{1, "marker_bit3"},
		{9, "system_clock_reference_extension"},
		{1, "marker_bit4"},
		{22, "program_mux_rate"},
		{1, "marker_bit5"},
		{1, "marker_bit6"},
		{5, "reserved"},
		{3, "pack_stuffing_length"},
	}
	return psDecoder
}

func main() {
	log.SetFlags(log.Lshortfile)
	psFile := os.Args[1]
	psBuf, err := ioutil.ReadFile(psFile)
	if err != nil {
		log.Printf("open file: %s error", psFile)
		return
	}
	log.Printf("file size: %d", len(psBuf))
	br := bitreader.NewReader(bytes.NewReader(psBuf))
	psDecoder := NewPsDecoder(br, &psBuf, len(psBuf))
	if err := psDecoder.decodePsPkts(); err != nil {
		log.Println(err)
		return
	}
	fmt.Println("")
	log.Printf("total frame count: %d\n", psDecoder.totalVideoFrameCnt)
	log.Printf("err frame cont: %d\n", psDecoder.errVideoFrameCnt)
	log.Printf("I frame count: %d\n", psDecoder.iFrameCnt)
	log.Printf("err I frame count: %d\n", psDecoder.errIFrameCnt)
	log.Printf("program stream map count: %d", psDecoder.psmCnt)
}
