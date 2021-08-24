package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
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
	ErrCheckInputFile    = errors.New("check input file error")
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
	errAudioFrameCnt   int
	totalVideoFrameCnt int
	totalAudioFrameCnt int
	iFrameCnt          int
	psmCnt             int
	errIFrameCnt       int
	pFrameCnt          int
	h264File           *os.File
	audioFile          *os.File
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
		dec.pFrameCnt++
	}
	if !err && dec.h264File != nil {
		dec.writeH264FrameToFile(data)
	}
	return nil
}

func (dec *PsDecoder) saveAudioPkt(data []byte, len uint32, err bool) error {
	log.Printf("\t\taudio len : %d", len)
	if !err && dec.audioFile != nil {
		dec.writeAudioFrameToFile(data)
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

// 移动到当前位置+payloadLen位置，判断startcode是否正确
// 如果startcode不正确，说明payloadLen是错误的
func (dec *PsDecoder) isPayloadLenValid(audioLen uint32) bool {
	psBuf := *dec.psBuf
	pos := dec.getPos() + int64(audioLen)
	packStartCode := binary.BigEndian.Uint32(psBuf[pos : pos+4])
	if !dec.isStartCodeValid(packStartCode) {
		log.Printf("check payload len error: 0x%x pos: %d", packStartCode, dec.getPos())
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

func (dec *PsDecoder) skipInvalidBytes(payloadLen uint32) error {
	dec.errAudioFrameCnt++
	log.Println("check audio error")
	br := dec.br
	pos, end := dec.GetNextPackPos()
	if !end {
		log.Printf("audio len err, expect: %d actual: %d",
			payloadLen, int64(pos)-dec.getPos())
		//log.Printf("% X\n", (*dec.psBuf)[pos:pos+32])
		log.Printf("skip pos: %d", pos)
		skipLen := pos - int(dec.getPos())
		log.Printf("skip len: %d", skipLen)
		skipBuf := make([]byte, skipLen)
		// 由于payloadLen是错误的，所以下一个startcode和当前位置之间的字节需要丢弃
		if _, err := io.ReadAtLeast(br, skipBuf, int(skipLen)); err != nil {
			log.Println(err)
			return err
		}
		dec.saveAudioPkt(skipBuf, uint32(skipLen), true)
		return ErrCheckH264
	}
	return nil
}

func (dec *PsDecoder) decodeAudioPes() error {
	log.Println("=== Audio ===")
	br := dec.br
	dec.totalAudioFrameCnt++
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
	payloadLen-- // payloadLen包含pesHeaderDataLen这个字段的长度，占用一个字节
	br.Skip(uint(pesHeaderDataLen * 8))
	payloadLen -= pesHeaderDataLen // payloadLen 包含pesHeaderDataLen的长度
	if !dec.isPayloadLenValid(payloadLen) {
		return dec.skipInvalidBytes(payloadLen)
	}
	payloadData := make([]byte, payloadLen) // 这里的payloadLen就是实际音频的长度了
	if _, err := io.ReadAtLeast(br, payloadData, int(payloadLen)); err != nil {
		return err
	}
	dec.saveAudioPkt(payloadData, payloadLen, false)

	return nil
}

func (dec *PsDecoder) decodeVideoPes() error {
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
			log.Printf("h264 len err, expect: %d actual: %d",
				payloadLen, int64(pos)-dec.getPos())
			//log.Printf("% X\n", (*dec.psBuf)[pos:pos+32])
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

func (dec *PsDecoder) writeH264FrameToFile(frame []byte) error {
	if _, err := dec.h264File.Write(frame); err != nil {
		log.Println(err)
		return err
	}
	dec.h264File.Sync()
	return nil
}

func (dec *PsDecoder) writeAudioFrameToFile(frame []byte) error {
	if _, err := dec.audioFile.Write(frame); err != nil {
		log.Println(err)
		return err
	}
	dec.audioFile.Sync()
	return nil
}

func (dec *PsDecoder) openVideoFile() error {
	file := "./output.video"
	var err error
	dec.h264File, err = os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (dec *PsDecoder) openAudioFile() error {
	file := "./output.audio"
	var err error
	dec.audioFile, err = os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func NewPsDecoder(br bitreader.BitReader, psBuf *[]byte, fileSize int, dumpAudio, dumpVideo bool) *PsDecoder {
	decoder := &PsDecoder{
		br:             br,
		psHeader:       make(map[string]uint32),
		handlers:       make(map[int]func() error),
		psHeaderFields: make([]FieldInfo, 14),
		fileSize:       fileSize,
		psBuf:          psBuf,
	}
	decoder.handlers = map[int]func() error{
		StartCodePS:    decoder.decodePsHeader,
		StartCodeSYS:   decoder.decodeSystemHeader,
		StartCodeMAP:   decoder.decodeProgramStreamMap,
		StartCodeVideo: decoder.decodeVideoPes,
		StartCodeAudio: decoder.decodeAudioPes,
	}
	decoder.psHeaderFields = []FieldInfo{
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
	if dumpAudio {
		err := decoder.openAudioFile()
		if err != nil {
			return nil
		}
	}
	if dumpVideo {
		err := decoder.openVideoFile()
		if err != nil {
			return nil
		}
	}
	return decoder
}

func (dec *PsDecoder) showInfo() {
	fmt.Println("")
	log.Printf("total frame count: %d\n", dec.totalVideoFrameCnt)
	log.Printf("err frame cont: %d\n", dec.errVideoFrameCnt)
	log.Printf("I frame count: %d\n", dec.iFrameCnt)
	log.Printf("err I frame count: %d\n", dec.errIFrameCnt)
	log.Printf("program stream map count: %d", dec.psmCnt)
	log.Printf("P frame count: %d\n", dec.pFrameCnt)
}

type consoleParam struct {
	psFile    string
	dumpAudio bool
	dumpVideo bool
}

func parseConsoleParam() (*consoleParam, error) {
	param := &consoleParam{}
	flag.StringVar(&param.psFile, "file", "", "input file")
	flag.BoolVar(&param.dumpAudio, "dump-audio", false, "dump audio")
	flag.BoolVar(&param.dumpVideo, "dump-video", false, "dump video")
	flag.Parse()
	if param.psFile == "" {
		log.Println("must input file")
		return nil, ErrCheckInputFile
	}
	return param, nil
}

func main() {
	log.SetFlags(log.Lshortfile)
	param, err := parseConsoleParam()
	if err != nil {
		return
	}
	psBuf, err := ioutil.ReadFile(param.psFile)
	if err != nil {
		log.Printf("open file: %s error", param.psFile)
		return
	}
	log.Printf("file size: %d", len(psBuf))
	br := bitreader.NewReader(bytes.NewReader(psBuf))
	decoder := NewPsDecoder(br, &psBuf, len(psBuf), param.dumpAudio, param.dumpVideo)
	if err := decoder.decodePsPkts(); err != nil {
		log.Println(err)
		return
	}
	decoder.showInfo()
}
