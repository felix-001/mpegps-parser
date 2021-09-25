package parser

import (
	"bitreader"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"gui"
	"io"
	"io/ioutil"
	"log"
	"os"
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

type PsDecoder struct {
	videoStreamType    uint32
	audioStreamType    uint32
	br                 bitreader.BitReader
	psHeader           map[string]uint32
	handlers           map[int]func() (*gui.TableItem, error)
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
	param              *consoleParam
	ch                 chan *gui.TableItem
}

func (dec *PsDecoder) decodePsPkts() error {
	for dec.getPos() < int64(dec.fileSize) {
		startCode, err := dec.br.Read32(32)
		if err != nil {
			log.Println(err)
			return err
		}
		dec.pktCnt++
		if dec.param.verbose {
			fmt.Println()
			log.Printf("pkt count: %d pos: %d/%d", dec.pktCnt, dec.getPos(), dec.fileSize)
		}
		handler, ok := dec.handlers[int(startCode)]
		if !ok {
			log.Printf("check startCode error: 0x%x pos:%d, fileSize:%d\n", startCode, dec.getPos(), dec.fileSize)
			return ErrParsePakcet
		}
		pos := dec.getPos()
		item, _ := handler()
		item.Offset = pos
		dec.ch <- item
	}
	return nil
}

func (dec *PsDecoder) decodeSystemHeader() (*gui.TableItem, error) {
	br := dec.br
	syslens, err := br.Read32(16)
	if dec.param.printSysHeader {
		log.Println("=== ps system header === ")
		log.Printf("\tsystem_header_length:%d", syslens)
	}
	item := &gui.TableItem{
		PktType: "system header",
		Status:  "OK",
	}
	if err != nil {
		item.Status = "Error"
		return item, err
	}

	br.Skip(uint(syslens) * 8)
	return item, nil
}

func (decoder *PsDecoder) getPos() int64 {
	pos := decoder.br.Size() - int64(decoder.br.Len())
	return pos
}

func (decoder *PsDecoder) decodePsmNLoop(programStreamMapLen uint32) error {
	br := decoder.br
	for programStreamMapLen > 0 {
		streamType, err := br.Read32(8)
		if decoder.param.printPsm {
			log.Printf("\t\tstream type: 0x%x", streamType)
		}
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
		if decoder.param.printPsm {
			log.Printf("\t\tstream id: 0x%x", elementaryStreamID)
		}
		elementaryStreamInfoLength, err := br.Read32(16)
		if err != nil {
			return err
		}
		if decoder.param.printPsm {
			log.Printf("\t\telementary_stream_info_length: %d", elementaryStreamInfoLength)
		}
		br.Skip(uint(elementaryStreamInfoLength * 8))
		programStreamMapLen -= (4 + elementaryStreamInfoLength)
	}
	return nil
}

func (dec *PsDecoder) decodeProgramStreamMap() (*gui.TableItem, error) {
	br := dec.br
	dec.psmCnt++
	item := &gui.TableItem{
		PktType: "program stream map",
		Status:  "Error",
	}
	psmLen, err := br.Read32(16)
	if err != nil {
		return item, err
	}
	if dec.param.printPsm {
		log.Println("=== program stream map ===")
		log.Printf("\tprogram_stream_map_length: %d pos: %d", psmLen, dec.getPos())
	}
	//drop psm version info
	br.Skip(16)
	psmLen -= 2
	programStreamInfoLen, err := br.Read32(16)
	if err != nil {
		return item, err
	}
	br.Skip(uint(programStreamInfoLen * 8))
	psmLen -= (programStreamInfoLen + 2)
	programStreamMapLen, err := br.Read32(16)
	if err != nil {
		return item, err
	}
	psmLen -= (2 + programStreamMapLen)
	if dec.param.printPsm {
		log.Printf("\tprogram_stream_info_length: %d", programStreamMapLen)
	}

	if err := dec.decodePsmNLoop(programStreamMapLen); err != nil {
		return item, err
	}

	// crc 32
	if psmLen != 4 {
		if dec.param.printPsm {
			log.Printf("psmLen: 0x%x", psmLen)
		}
		return item, ErrFormatPack
	}
	br.Skip(32)
	item.Status = "OK"
	return item, nil
}

func (dec *PsDecoder) decodeH264(data []byte, len uint32, err bool) error {
	if dec.param.verbose {
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
	}
	if !err && dec.h264File != nil {
		dec.writeH264FrameToFile(data)
	}
	return nil
}

func (dec *PsDecoder) saveAudioPkt(data []byte, len uint32, err bool) error {
	if dec.param.verbose {
		log.Printf("\t\taudio len : %d", len)
	}
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
	psBuf := *dec.psBuf
	pos := dec.getPos() + int64(payloadLen)
	if pos >= int64(dec.fileSize) {
		log.Println("reach file end, quit")
		return false
	}
	packStartCode := binary.BigEndian.Uint32(psBuf[pos : pos+4])
	if !dec.isStartCodeValid(packStartCode) {
		log.Printf("check payload len error, len: %d pes start pos: %d(0x%x), pesType:%d", payloadLen, pesStartPos, pesStartPos, pesType)
		return false
	}
	return true
}

func (dec *PsDecoder) GetNextPackPos() int {
	pos := int(dec.getPos())
	for pos < dec.fileSize-4 {
		b := (*dec.psBuf)[pos : pos+4]
		packStartCode := binary.BigEndian.Uint32(b)
		if dec.isStartCodeValid((packStartCode)) {
			return pos
		}
		pos++
	}
	return dec.fileSize
}

func (dec *PsDecoder) skipInvalidBytes(payloadLen uint32, pesType int, pesStartPos int64) error {
	if pesType == VideoPES {
		dec.errVideoFrameCnt++
	} else {
		dec.errAudioFrameCnt++
	}
	br := dec.br
	pos := dec.GetNextPackPos()
	skipLen := pos - int(dec.getPos())
	log.Printf("pes start dump: % X\n", (*dec.psBuf)[pesStartPos:pesStartPos+16])
	log.Printf("pes payload len err, expect: %d actual: %d", payloadLen, skipLen)
	log.Printf("skip len: %d, next pack pos:%d", skipLen, pos)
	skipBuf := make([]byte, skipLen)
	// 由于payloadLen是错误的，所以下一个startcode和当前位置之间的字节需要丢弃
	if _, err := io.ReadAtLeast(br, skipBuf, int(skipLen)); err != nil {
		log.Println(err)
		return err
	}
	if pesType == AudioPES {
		dec.saveAudioPkt(skipBuf, uint32(skipLen), true)
	} else {
		dec.decodeH264(skipBuf, uint32(skipLen), true)
	}
	return nil
}

func (dec *PsDecoder) decodeAudioPes() (*gui.TableItem, error) {
	if dec.param.verbose {
		log.Println("=== Audio ===")
	}
	dec.totalAudioFrameCnt++
	item := &gui.TableItem{
		PktType: "audio pes",
		Status:  "Error",
	}
	err := dec.decodePES(AudioPES)
	if err != nil {
		return item, err
	}
	item.Status = "OK"
	return item, nil
}

func (dec *PsDecoder) decodePESHeader() (uint32, error) {
	br := dec.br
	/* payload length */
	payloadLen, err := br.Read32(16)
	if err != nil {
		log.Println(err)
		return 0, err
	}

	/* flags: pts_dts_flags ... */
	br.Skip(16) // 跳过各种flags,比如pts_dts_flags
	payloadLen -= 2

	/* pes header data length */
	pesHeaderDataLen, err := br.Read32(8)
	if err != nil {
		log.Println(err)
		return 0, err
	}
	if dec.param.verbose {
		log.Printf("\tPES_packet_length: %d", payloadLen)
		log.Printf("\tpes_header_data_length: %d", pesHeaderDataLen)
	}
	payloadLen--

	/* pes header data */
	br.Skip(uint(pesHeaderDataLen * 8))
	payloadLen -= pesHeaderDataLen
	return payloadLen, nil
}

func (dec *PsDecoder) decodePES(pesType int) error {
	br := dec.br
	pesStartPos := dec.getPos() - 4 // 4为startcode的长度
	if dec.param.dumpPesStartBytes {
		log.Printf("% X\n", (*dec.psBuf)[pesStartPos:pesStartPos+16])
	}
	payloadLen, err := dec.decodePESHeader()
	if err != nil {
		return err
	}
	if !dec.isPayloadLenValid(payloadLen, pesType, pesStartPos) {
		return dec.skipInvalidBytes(payloadLen, pesType, pesStartPos)
	}
	payloadData := make([]byte, payloadLen)
	if _, err := io.ReadAtLeast(br, payloadData, int(payloadLen)); err != nil {
		return err
	}
	if pesType == VideoPES {
		dec.decodeH264(payloadData, payloadLen, false)
	} else {
		dec.saveAudioPkt(payloadData, payloadLen, false)
	}

	return nil
}

func (dec *PsDecoder) decodeVideoPes() (*gui.TableItem, error) {
	if dec.param.verbose {
		log.Println("=== video ===")
	}
	dec.totalVideoFrameCnt++
	item := &gui.TableItem{
		PktType: "video pes",
		Status:  "Error",
	}
	err := dec.decodePES(VideoPES)
	if err != nil {
		return item, err
	}
	item.Status = "OK"
	return item, nil
}

func (decoder *PsDecoder) decodePsHeader() (*gui.TableItem, error) {
	if decoder.param.verbose {
		log.Println("=== pack header ===")
	}
	item := &gui.TableItem{
		PktType: "pack header",
		Status:  "OK",
	}
	psHeaderFields := decoder.psHeaderFields
	for _, field := range psHeaderFields {
		val, err := decoder.br.Read32(field.len)
		if err != nil {
			log.Printf("parse %s error", field.item)
			item.Status = "Error"
			return item, err
		}
		decoder.psHeader[field.item] = val
	}
	pack_stuffing_length := decoder.psHeader["pack_stuffing_length"]
	decoder.br.Skip(uint(pack_stuffing_length * 8))
	if decoder.param.printPsHeader {
		b, err := json.MarshalIndent(decoder.psHeader, "", "  ")
		if err != nil {
			log.Println("error:", err)
		}
		fmt.Print(string(b) + "\n")
	}
	return item, nil
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
	dec.h264File, err = os.OpenFile(dec.param.outputVideoFile, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (dec *PsDecoder) openAudioFile() error {
	var err error
	dec.audioFile, err = os.OpenFile(dec.param.outputAudioFile, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func NewPsDecoder(br bitreader.BitReader, psBuf *[]byte, fileSize int, param *consoleParam, ch chan *gui.TableItem) *PsDecoder {
	decoder := &PsDecoder{
		br:             br,
		psHeader:       make(map[string]uint32),
		handlers:       make(map[int]func() (*gui.TableItem, error)),
		psHeaderFields: make([]FieldInfo, 14),
		fileSize:       fileSize,
		psBuf:          psBuf,
		param:          param,
		ch:             ch,
	}
	decoder.handlers = map[int]func() (*gui.TableItem, error){
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
	if param.dumpAudio {
		err := decoder.openAudioFile()
		if err != nil {
			return nil
		}
	}
	if param.dumpVideo {
		err := decoder.openVideoFile()
		if err != nil {
			return nil
		}
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

type consoleParam struct {
	psFile            string
	outputAudioFile   string
	outputVideoFile   string
	dumpAudio         bool
	dumpVideo         bool
	printPsHeader     bool
	printSysHeader    bool
	printPsm          bool
	verbose           bool
	dumpPesStartBytes bool
}

func parseConsoleParam() (*consoleParam, error) {
	param := &consoleParam{}
	flag.StringVar(&param.psFile, "file", "", "input file")
	flag.StringVar(&param.outputAudioFile, "output-audio", "./output.audio", "output audio file")
	flag.StringVar(&param.outputVideoFile, "output-video", "./output.video", "output video file")
	flag.BoolVar(&param.dumpAudio, "dump-audio", false, "dump audio")
	flag.BoolVar(&param.dumpVideo, "dump-video", false, "dump video")
	flag.BoolVar(&param.printPsHeader, "print-ps-header", false, "print ps header")
	flag.BoolVar(&param.printSysHeader, "print-sys-header", false, "print system header")
	flag.BoolVar(&param.printPsm, "print-psm", false, "print porgram stream map")
	flag.BoolVar(&param.verbose, "verbose", false, "show packet detail")
	flag.BoolVar(&param.dumpPesStartBytes, "dump-pes-start-bytes", false, "dump pes start bytes")
	flag.Parse()
	if param.psFile == "" {
		log.Println("must input file")
		return nil, ErrCheckInputFile
	}
	return param, nil
}

func Process(ch chan *gui.TableItem) {
	param, err := parseConsoleParam()
	if err != nil {
		return
	}
	psBuf, err := ioutil.ReadFile(param.psFile)
	if err != nil {
		log.Printf("open file: %s error", param.psFile)
		return
	}
	log.Println(param.psFile, "file size:", len(psBuf))
	br := bitreader.NewReader(bytes.NewReader(psBuf))
	decoder := NewPsDecoder(br, &psBuf, len(psBuf), param, ch)
	if err := decoder.decodePsPkts(); err != nil {
		log.Println(err)
		return
	}
	decoder.showInfo()
}
