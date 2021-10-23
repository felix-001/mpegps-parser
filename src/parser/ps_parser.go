package parser

import (
	"bitreader"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"ntree"
	"os"
	"param"
	"ui"
)

const (
	StartCodePS    = 0x000001ba
	StartCodeSYS   = 0x000001bb
	StartCodeMAP   = 0x000001bc
	StartCodeVideo = 0x000001e0
	StartCodeAudio = 0x000001c0
)

const (
	VideoPES = 0x01
	AudioPES = 0x02
)

var (
	ErrNotFoundStartCode = errors.New("not found the need start code flag")
	ErrFormatPack        = errors.New("not package standard")
	ErrParsePakcet       = errors.New("parse ps packet error")
	ErrNewBiteReader     = errors.New("new bit reader error")
	ErrCheckH264         = errors.New("check h264 error")
	ErrCheckPayloadLen   = errors.New("check payload length error")
	ErrCheckInputFile    = errors.New("check input file error")
)

type FieldInfo struct {
	len  uint
	item string
}

type ItemInfo struct {
	key   string
	value string
}

type PktInfo struct {
	Type   string
	Status string
	Offset int
	Detail *ntree.NTree
}

type PsDecoder struct {
	videoStreamType    uint32
	audioStreamType    uint32
	br                 *bitreader.BitReader
	pktCnt             int
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
	param              *param.ConsoleParam
	ch                 chan *ui.TableItem
}

func (decoder *PsDecoder) decodePkt(startCode uint32) (typ string, tree *ntree.NTree, err error) {
	switch startCode {
	case StartCodePS:
		typ = "pack header"
		tree, err = decoder.decodePsHeader()
	case StartCodeSYS:
		typ = "system header"
		tree, err = decoder.decodeSystemHeader()
	case StartCodeMAP:
		typ = "program stream map"
		tree, err = decoder.decodeProgramStreamMap()
	case StartCodeVideo:
		typ = "video pes"
		tree, err = decoder.decodeVideoPes()
	case StartCodeAudio:
		typ = "audio pes"
		tree, err = decoder.decodeAudioPes()
	default:
		err = ErrNotFoundStartCode
	}
	return
}

func (decoder *PsDecoder) sendBasic(startCode uint32, typ string, status string) {
	if startCode == StartCodePS {
		return
	}
	offset, _ := decoder.br.Offset()
	item := &ui.TableItem{
		Offset:  int64(offset),
		PktType: typ,
		Status:  status,
	}
	decoder.ch <- item
}

func (decoder *PsDecoder) decodePkts() error {
	br := decoder.br
	// todo 这里offset不能这样判断
	offset, _ := br.Offset()
	for offset < br.Size() {
		startCode, err := br.Read(32)
		if err != nil {
			log.Println(err)
			return err
		}
		decoder.pktCnt++
		typ, _, err := decoder.decodePkt(uint32(startCode))
		status := "OK"
		if err != nil {
			status = "Error"
		}
		decoder.sendBasic(uint32(startCode), typ, status)
		offset, _ = br.Offset()
	}
	return nil
}

func (dec *PsDecoder) decodeSystemHeader() (*ntree.NTree, error) {
	br := dec.br
	syslens, err := br.Read(16)
	if err != nil {
		return nil, err
	}
	b := make([]byte, syslens)
	br.ReadBytes(b)
	return nil, nil
}

func (decoder *PsDecoder) decodePsmNLoop(programStreamMapLen uint32) error {
	br := decoder.br
	for programStreamMapLen > 0 {
		streamType, err := br.Read(8)
		if err != nil {
			return err
		}
		elementaryStreamID, err := br.Read(8)
		if err != nil {
			return err
		}
		if elementaryStreamID >= 0xe0 && elementaryStreamID <= 0xef {
			decoder.videoStreamType = uint32(streamType)
		}
		if elementaryStreamID >= 0xc0 && elementaryStreamID <= 0xdf {
			decoder.audioStreamType = uint32(streamType)
		}
		elementaryStreamInfoLength, err := br.Read(16)
		if err != nil {
			return err
		}
		b := make([]byte, elementaryStreamInfoLength)
		br.ReadBytes(b)
		programStreamMapLen -= (4 + uint32(elementaryStreamInfoLength))
	}
	return nil
}

func (dec *PsDecoder) decodeProgramStreamMap() (*ntree.NTree, error) {
	br := dec.br
	dec.psmCnt++

	psmLen, err := br.Read(16)
	if err != nil {
		return nil, err
	}
	//drop psm version info
	br.Read(16)
	psmLen -= 2
	programStreamInfoLen, err := br.Read(16)
	if err != nil {
		return nil, err
	}
	b := make([]byte, programStreamInfoLen)
	br.ReadBytes(b)
	psmLen -= (programStreamInfoLen + 2)
	programStreamMapLen, err := br.Read(16)
	if err != nil {
		return nil, err
	}
	psmLen -= (2 + programStreamMapLen)

	if err := dec.decodePsmNLoop(uint32(programStreamMapLen)); err != nil {
		return nil, err
	}

	// crc 32
	if psmLen != 4 {
		return nil, ErrFormatPack
	}
	br.Read(32)
	return nil, nil
}

func (dec *PsDecoder) decodeH264(data []byte, len uint32, err bool) error {
	if dec.param.Verbose {
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
	}
	if !err && dec.h264File != nil {
		dec.writeH264FrameToFile(data)
	}
	return nil
}

func (dec *PsDecoder) saveAudioPkt(data []byte, len uint32, err bool) error {
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

// 移动到当前位置+payloadLen位置，判断startcode是否正确
// 如果startcode不正确，说明payloadLen是错误的
func (dec *PsDecoder) isPayloadLenValid(payloadLen uint32, pesType int, pesStartPos int64) bool {
	br := dec.br
	offset, _ := br.Offset()
	pos := offset + int64(payloadLen)
	if pos >= br.Size() {
		log.Println("reach file end, quit")
		return false
	}
	buf := make([]byte, 4, 4)
	if _, err := br.ReadAt(buf, pos); err != nil {
		return false
	}
	offset, _ = br.Offset()
	packStartCode := binary.BigEndian.Uint32(buf)
	if !dec.isStartCodeValid(packStartCode) {
		log.Printf("check payload len error, len: %d pes start pos: %d(0x%x), pesType:%d", payloadLen, pesStartPos, pesStartPos, pesType)
		return false
	}
	return true
}

func (dec *PsDecoder) GetNextPackPos() int64 {
	br := dec.br
	offset, _ := br.Offset()
	for offset < br.Size()-4 {
		buf := make([]byte, 4, 4)
		if _, err := br.ReadAt(buf, offset); err != nil {
			return 0
		}
		packStartCode := binary.BigEndian.Uint32(buf)
		if dec.isStartCodeValid((packStartCode)) {
			return offset
		}
		offset++
	}
	return br.Size()
}

func (dec *PsDecoder) ReadInvalidBytes(payloadLen uint32, pesType int, pesStartPos int64) error {
	if pesType == VideoPES {
		dec.errVideoFrameCnt++
	} else {
		dec.errAudioFrameCnt++
	}
	br := dec.br
	pos := dec.GetNextPackPos()
	offset, _ := br.Offset()
	readLen := pos - offset
	log.Printf("pes payload len err, expect: %d actual: %d", payloadLen, readLen)
	log.Printf("Read len: %d, next pack pos:%d", readLen, pos)
	readBuf := make([]byte, readLen)
	// 由于payloadLen是错误的，所以下一个startcode和当前位置之间的字节需要丢弃
	if _, err := br.ReadBytes(readBuf); err != nil {
		log.Println(err)
		return err
	}
	if pesType == AudioPES {
		dec.saveAudioPkt(readBuf, uint32(readLen), true)
	} else {
		dec.decodeH264(readBuf, uint32(readLen), true)
	}
	return nil
}

func (dec *PsDecoder) decodeAudioPes() (*ntree.NTree, error) {
	dec.totalAudioFrameCnt++
	err := dec.decodePES(AudioPES)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (dec *PsDecoder) decodePESHeader() (uint32, error) {
	br := dec.br
	/* payload length */
	payloadLen, err := br.Read(16)
	if err != nil {
		log.Println(err)
		return 0, err
	}

	/* flags: pts_dts_flags ... */
	br.Read(16) // 跳过各种flags,比如pts_dts_flags
	payloadLen -= 2

	/* pes header data length */
	pesHeaderDataLen, err := br.Read(8)
	if err != nil {
		log.Println(err)
		return 0, err
	}
	payloadLen--

	//log.Println("pesHeaderDataLen", pesHeaderDataLen)
	/* pes header data */
	b := make([]byte, pesHeaderDataLen)
	br.ReadBytes(b)
	payloadLen -= pesHeaderDataLen
	return uint32(payloadLen), nil
}

func (dec *PsDecoder) decodePES(pesType int) error {
	br := dec.br
	offset, _ := br.Offset()
	pesStartPos := offset - 4 // 4为startcode的长度
	payloadLen, err := dec.decodePESHeader()
	if err != nil {
		return err
	}
	offset, _ = br.Offset()
	if !dec.isPayloadLenValid(payloadLen, pesType, pesStartPos) {
		dec.ReadInvalidBytes(payloadLen, pesType, pesStartPos)
		return ErrCheckPayloadLen
	}
	payloadData := make([]byte, payloadLen)
	if _, err := br.ReadBytes(payloadData); err != nil {
		log.Println(err)
		return err
	}
	offset, _ = br.Offset()
	if pesType == VideoPES {
		dec.decodeH264(payloadData, payloadLen, false)
	} else {
		dec.saveAudioPkt(payloadData, payloadLen, false)
	}

	return nil
}

func (dec *PsDecoder) decodeVideoPes() (*ntree.NTree, error) {
	dec.totalVideoFrameCnt++
	err := dec.decodePES(VideoPES)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (decoder *PsDecoder) decodePsHeader() (*ntree.NTree, error) {
	psHeaderFields := []FieldInfo{
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
	psHeaders := map[string]uint64{}
	for _, field := range psHeaderFields {
		val, err := decoder.br.Read(field.len)
		if err != nil {
			log.Printf("parse %s error", field.item)
			return nil, err
		}
		psHeaders[field.item] = val
	}
	pack_stuffing_length := psHeaders["pack_stuffing_length"]
	decoder.br.Read(uint(pack_stuffing_length * 8))
	return nil, nil
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
	var err error
	dec.h264File, err = os.OpenFile(dec.param.OutputVideoFile, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (dec *PsDecoder) openAudioFile() error {
	var err error
	dec.audioFile, err = os.OpenFile(dec.param.OutputAudioFile, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (decoder *PsDecoder) ParseDetail(offset int, typ string) (*ntree.NTree, error) {
	switch typ {
	case "video pes":
		return decoder.decodeVideoPes()
	case "audio pes":
		return decoder.decodeAudioPes()
	case "program stream map":
		return decoder.decodeProgramStreamMap()
	}
	return nil, nil
}

func (dec *PsDecoder) Run() {
	go dec.decodePkts()
}

func New(param *param.ConsoleParam, ch chan *ui.TableItem) *PsDecoder {
	f, err := os.Open(param.PsFile)
	if err != nil {
		log.Println(err)
		return nil
	}
	fileInfo, err := os.Stat(param.PsFile)
	if err != nil {
		log.Println(err)
		return nil
	}
	br := bitreader.New(f, fileInfo.Size())
	decoder := &PsDecoder{
		br:    br,
		param: param,
		ch:    ch,
	}
	return decoder
}

func (dec *PsDecoder) showInfo() {
	fmt.Println()
	log.Printf("total video frame count: %d\n", dec.totalVideoFrameCnt)
	log.Printf("err frame cont: %d\n", dec.errVideoFrameCnt)
	log.Printf("I frame count: %d\n", dec.iFrameCnt)
	log.Printf("err I frame count: %d\n", dec.errIFrameCnt)
	log.Printf("program stream map count: %d", dec.psmCnt)
	log.Printf("P frame count: %d\n", dec.pFrameCnt)
	log.Println("total audio frame count:", dec.totalAudioFrameCnt)
	log.Printf("video stream type: 0x%x\n", dec.videoStreamType)
	log.Printf("audio stream type: 0x%x\n", dec.audioStreamType)
}
