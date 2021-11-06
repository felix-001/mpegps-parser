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
	"reader"
)

const (
	StartCodePS    = 0x000001ba
	StartCodeSYS   = 0x000001bb
	StartCodeMAP   = 0x000001bc
	StartCodeVideo = 0x000001e0
	StartCodeAudio = 0x000001c0
)

const (
	fast_forward = 0
	slow_motion  = 1
	freeze_frame = 2
	fast_reverse = 3
	slow_reverse = 4
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
	ErrCheckLength       = errors.New("check length error")
)

// map遍历是无序的
type Item struct {
	k string
	v uint64
}

type Items []Item

var (
	packHeader = Items{
		{"fixed", 2},
		{"system_clock_reference_base1", 3},
		{"marker_bit1", 1},
		{"system_clock_reference_base2", 15},
		{"marker_bit2", 1},
		{"system_clock_reference_base3", 15},
		{"marker_bit3", 1},
		{"system_clock_reference_extension", 9},
		{"marker_bit4", 1},
		{"program_mux_rate", 22},
		{"marker_bit5", 1},
		{"marker_bit6", 1},
		{"resvrved", 5},
		{"pack_stuffing_length", 3},
	}
	systemHeader = Items{
		{"header_length", 16},
		{"marker_bit1", 1},
		{"rate_bound", 22},
		{"marker_bit2", 1},
		{"audio_bound", 6},
		{"fixed_flag", 1},
		{"CSPS_flag", 1},
		{"system_audio_lock_flag", 1},
		{"system_video_lock_flag", 1},
		{"marker_bit3", 1},
		{"video_bound", 5},
		{"packet_rate_restriction_flag", 1},
		{"reserved_bits", 7},
	}
	systemHeaderDetail = Items{
		{"stream_id", 8},
		{"fixed", 2},
		{"P-STD_buffer_bound_scale", 1},
		{"P-STD_buffer_size_bound", 13},
	}
	programStreamMap = Items{
		{"program_stream_map_length", 16},
		{"current_next_indicator", 1},
		{"reserved1", 2},
		{"program_stream_map_version", 5},
		{"reserved2", 7},
		{"marker_bit", 1},
		{"program_stream_info_length", 16},
	}
	elementaryStreamMap = Items{
		{"stream_type", 8},
		{"elementary_stream_id", 8},
		{"elementary_stream_info_length", 16},
	}
	ptsInfo = Items{
		{"fixed", 4},
		{"PTS1", 3},
		{"marker_bit1", 1},
		{"PTS2", 15},
		{"marker_bit2", 1},
		{"PTS3", 15},
		{"marker_bit3", 1},
	}
	ptsdtsInfo = Items{
		{"fixed1", 4},
		{"PTS1", 3},
		{"marker_bit1", 1},
		{"PTS2", 15},
		{"marker_bit2", 1},
		{"PTS3", 15},
		{"marker_bit3", 1},
		{"fixed2", 4},
		{"DTS1", 3},
		{"marker_bit4", 1},
		{"DTS2", 15},
		{"marker_bit5", 1},
		{"DTS3", 15},
		{"marker_bit6", 1},
	}
	escrInfo = Items{
		{"reserved", 1},
		{"ESCR_base1", 3},
		{"marker_bit1", 1},
		{"ESCR_base2", 15},
		{"marker_bit2", 1},
		{"ESCR_base3", 15},
		{"marker_bit3", 1},
		{"ESCR_extension", 9},
		{"marker_bit4", 1},
	}
	esRateInfo = Items{
		{"marker_bit1", 1},
		{"ES_rate", 22},
		{"marker_bit2", 1},
	}
	pes = Items{
		{"PES_packet_length", 16},
		{"fixed", 2},
		{"PES_scrambling_control", 2},
		{"PES_priority", 1},
		{"data_alignment_indicator", 1},
		{"copyright", 1},
		{"original_or_copy", 1},
		{"PTS_DTS_flags", 2},
		{"ESCR_flag", 1},
		{"ES_rate_flag", 1},
		{"DSM_trick_mode_flag", 1},
		{"additional_copy_info_flag", 1},
		{"PES_CRC_flag", 1},
		{"PES_extension_flag", 1},
		{"PES_header_data_length", 8},
	}
	pesExt = Items{
		{"PES_private_data_flag", 1},
		{"pack_header_field_flag", 1},
		{"program_packet_sequence_counter_flag", 1},
		{"P-STD_buffer_flag", 1},
		{"reserved", 3},
		{"PES_extension_flag_2", 1},
	}
	dsmFastInfo = Items{
		{"field_id", 2},
		{"intra_slice_refresh", 1},
		{"frequency_truncation", 2},
	}
	sequenceCount = Items{
		{"marker_bit1", 1},
		{"program_packet_sequence_counter", 7},
		{"marker_bit2", 1},
		{"MPEG1_MPEG2_identifier", 1},
		{"original_stuff_length", 6},
	}
	pstdBuf = Items{
		{"fixed", 2},
		{"P-STD_buffer_scale", 1},
		{"P-STD_buffer_size", 13},
	}
)

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
	ch                 chan *reader.PktInfo
}

func (decoder *PsDecoder) decodePkt(startCode uint32) (typ string, t *ntree.NTree, err error) {
	switch startCode {
	case StartCodePS:
		typ = "pack header"
		t, err = decoder.decodePsHeader()
	case StartCodeSYS:
		typ = "system header"
		t, err = decoder.decodeSystemHeader()
		offset, _ := decoder.br.Offset()
		log.Println("StartCodeSYS", offset)
	case StartCodeMAP:
		typ = "program stream map"
		t, err = decoder.decodeProgramStreamMap()
	case StartCodeVideo:
		typ = "video pes"
		t, err = decoder.decodeVideoPes()
	case StartCodeAudio:
		typ = "audio pes"
		t, err = decoder.decodeAudioPes()
	default:
		err = ErrNotFoundStartCode
	}
	return
}

func (decoder *PsDecoder) sendBasic(startCode uint32, typ, status string) {
	if startCode == StartCodePS {
		return
	}
	offset, _ := decoder.br.Offset()
	pktInfo := &reader.PktInfo{
		Offset: uint64(offset),
		Typ:    typ,
		Status: status,
	}
	decoder.ch <- pktInfo
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
		//log.Println("offset:", offset)
		typ, _, err := decoder.decodePkt(uint32(startCode))
		if err != nil && err != ErrCheckPayloadLen {
			offset, _ := br.Offset()
			log.Println(err, startCode, "offset:", offset)
			return err
		}
		status := "OK"
		if err != nil {
			status = "Error"
		}
		decoder.sendBasic(uint32(startCode), typ, status)
		offset, _ = br.Offset()
	}
	decoder.showInfo()
	return nil
}

func (dec *PsDecoder) decodeSystemHeader() (*ntree.NTree, error) {
	t := ntree.New(&Item{k: "system header"})
	dm := NewDataManager(dec.br, t)
	dm.decode(systemHeader)
	nextbits, err := dec.br.Peek(1)
	if err != nil {
		return nil, err
	}
	for nextbits == 1 {
		dm.decode(systemHeaderDetail)
		if nextbits, err = dec.br.Peek(1); err != nil {
			return nil, err
		}
	}
	//dump(dm.tree)
	return t, nil
}

func (dm *DataManager) skipBytes(size uint64) {
	buf := make([]byte, size)
	dm.br.ReadBytes(buf)
}

func (dec *PsDecoder) decodeProgramStreamMap() (*ntree.NTree, error) {
	t := ntree.New(&Item{k: "program stream map"})
	dm := NewDataManager(dec.br, t)
	if err := dm.decode(programStreamMap); err != nil {
		return nil, err
	}
	dm.skipBytes(dm.getData("program_stream_info_length"))

	esTree := ntree.New(&Item{k: "elementary stream map"})
	t.Append(esTree)
	elementary_stream_map_length := dm.read("elementary_stream_map_length", 16)
	//dm.dump()
	//log.Println("elementary_stream_map_length", elementary_stream_map_length)
	for elementary_stream_map_length > 0 {
		if err := dm.decodeChild(elementaryStreamMap, esTree); err != nil {
			return nil, err
		}
		elementary_stream_info_length := dm.getDataFromTree(esTree, "elementary_stream_info_length")
		//`log.Println("elementary_stream_info_length", elementary_stream_info_length)
		dm.skipBytes(elementary_stream_info_length)
		if elementary_stream_info_length+4 > elementary_stream_map_length {
			dm.dump()
			log.Println("elementary_stream_info_length:", elementary_stream_info_length,
				"elementary_stream_map_length:", elementary_stream_map_length)
			return nil, ErrCheckLength
		}
		elementary_stream_map_length -= (4 + elementary_stream_info_length)
		log.Println("elementary_stream_map_length", elementary_stream_map_length)
	}
	dm.read("CRC_32", 32)
	return t, nil
}

func (dec *PsDecoder) decodeH264(data []byte) error {
	if dec.param.Verbose {
		if data[4] == 0x67 {
			log.Println("\t\tSPS")
		}
		if data[4] == 0x68 {
			log.Println("\t\tPPS")
		}
		if data[4] == 0x65 {
			log.Println("\t\tIDR")
		}
		if data[4] == 0x61 {
			log.Println("\t\tP Frame")
			dec.pFrameCnt++
		}
	}
	// TODO: 命令行参数设置错误的帧是否写入文件
	if dec.h264File != nil {
		dec.writeH264FrameToFile(data)
	}
	return nil
}

func (dec *PsDecoder) saveAudioPkt(data []byte) error {
	// TODO: 命令行参数可以设置错误的帧是否写入文件
	if dec.audioFile != nil {
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
func (dec *PsDecoder) isPayloadLenValid(payloadLen uint64, pesType int, pesStartPos int64) bool {
	br := dec.br
	offset, _ := br.Offset()
	pos := offset + int64(payloadLen)
	if pos >= br.Size() {
		log.Println("reach file end, quit")
		return false
	}
	buf := make([]byte, 4)
	if _, err := br.ReadAt(buf, pos); err != nil {
		return false
	}
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
		buf := make([]byte, 4)
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

func (dec *PsDecoder) ReadInvalidBytes(payloadLen uint64, pesType int, pesStartPos int64) ([]byte, error) {
	if pesType == VideoPES {
		dec.errVideoFrameCnt++
	} else {
		dec.errAudioFrameCnt++
	}
	offset, _ := dec.br.Offset()
	readLen := dec.GetNextPackPos() - offset
	log.Printf("pes payload len err, expect: %d actual: %d", payloadLen, readLen)
	log.Printf("Read len: %d, next pack pos:%d", readLen, dec.GetNextPackPos())
	// 由于payloadLen是错误的，所以下一个startcode和当前位置之间的字节需要丢弃
	readBuf, err := dec.readBytes(uint64(readLen))
	return readBuf, err
}

func (dec *PsDecoder) getPesPayloadLen(dm *DataManager) uint64 {
	payloadLen := dm.getData("PES_packet_length")
	PES_header_data_length := dm.getData("PES_header_data_length")
	// 3 - 各种flag + PES_header_data_length本身的8个bit
	//log.Println("PES_header_data_length", PES_header_data_length)
	return payloadLen - 3 - PES_header_data_length
}

func (dec *PsDecoder) decodeAudioPes() (*ntree.NTree, error) {
	dec.totalAudioFrameCnt++
	t := ntree.New(&Item{k: "audio pes"})
	dm := NewDataManager(dec.br, t)
	payload, err := dec.decodePES(AudioPES, dm)
	if err != nil {
		return nil, err
	}
	dec.saveAudioPkt(payload)
	return t, nil
}

func (dec *PsDecoder) decodeDSMTrickMode(dm *DataManager) error {

	trick_mode_control, _ := dec.br.Read(3)
	dm.set("trick_mode_control", trick_mode_control)
	switch trick_mode_control {
	case fast_forward:
	case fast_reverse:
		dm.decode(dsmFastInfo)
	case slow_motion:
	case slow_reverse:
		rep_cntrl, _ := dec.br.Read(5)
		dm.set("rep_cntrl", rep_cntrl)
	default:
		dec.br.Read(5)

	}
	return nil
}

func (dec *PsDecoder) decodePesExtension(dm *DataManager) error {
	dm.decode(pesExt)
	if dm.getData("PES_private_data_flag") == 1 {
		dm.skipBytes(16)
	}
	if dm.getData("pack_header_field_flag") == 1 {
		pack_field_length := dm.read("pack_field_length", 8)
		dm.skipBytes(pack_field_length)
	}
	if dm.getData("program_packet_sequence_counter_flag") == 1 {
		dm.decode(sequenceCount)
	}
	if dm.getData("P-STD_buffer_flag") == 1 {
		dm.decode(pstdBuf)
	}
	if dm.getData("PES_extension_flag_2") == 1 {
		dec.br.Read(1)
		PES_extension_field_length := dm.read("PES_extension_field_length", 7)
		dm.skipBytes(PES_extension_field_length)
	}
	return nil
}

func (dec *PsDecoder) decodePESHeader(dm *DataManager) error {
	if err := dm.decode(pes); err != nil {
		return err
	}
	start, _ := dec.br.Offset()
	if dm.getData("PTS_DTS_flags") == 2 {
		dm.decode(ptsInfo)
	}
	if dm.getData("PTS_DTS_flags") == 3 {
		dm.decode(ptsdtsInfo)
	}
	if dm.getData("ESCR_flag") == 1 {
		dm.decode(escrInfo)
	}
	if dm.getData("ES_rate_flag") == 1 {
		dm.decode(esRateInfo)
	}
	if dm.getData("DSM_trick_mode_flag") == 1 {
		dec.decodeDSMTrickMode(dm)
	}
	if dm.getData("additional_copy_info_flag") == 1 {
		dec.br.Read(1)
		additional_copy_info, _ := dec.br.Read(7)
		dm.set("additional_copy_info", additional_copy_info)
	}
	if dm.getData("PES_CRC_flag") == 1 {
		previous_PES_packet_CRC, _ := dec.br.Read(16)
		dm.set("previous_PES_packet_CRC", previous_PES_packet_CRC)
	}
	if dm.getData("PES_extension_flag") == 1 {
		dec.decodePesExtension(dm)
	}
	end, _ := dec.br.Offset()
	nb_bytes := end - start
	if dm.getData("PES_header_data_length") > uint64(nb_bytes) {
		nb_stuffing_bytes := dm.getData("PES_header_data_length") - uint64(nb_bytes)
		dec.readBytes(nb_stuffing_bytes)
	}
	//fmt.Printf("%+v", dm.tree)
	//dump(dm.tree)
	return nil
}

func (dec *PsDecoder) readBytes(size uint64) ([]byte, error) {
	buf := make([]byte, size)
	if _, err := dec.br.ReadBytes(buf); err != nil {
		log.Println(err)
		return nil, err
	}
	return buf, nil
}

func (dec *PsDecoder) decodePES(pesType int, dm *DataManager) ([]byte, error) {
	offset, _ := dec.br.Offset()
	pesStartPos := offset - 4 // 4为startcode的长度
	if err := dec.decodePESHeader(dm); err != nil {
		return nil, err
	}
	payloadLen := dec.getPesPayloadLen(dm)
	if !dec.isPayloadLenValid(payloadLen, pesType, pesStartPos) {
		payload, err := dec.ReadInvalidBytes(payloadLen, pesType, pesStartPos)
		if err != nil {
			return nil, err
		}
		return payload, ErrCheckPayloadLen
	}
	return dec.readBytes(payloadLen)
}

func (dec *PsDecoder) decodeVideoPes() (*ntree.NTree, error) {
	dec.totalVideoFrameCnt++
	t := ntree.New(&Item{k: "video pes"})
	dm := NewDataManager(dec.br, t)
	payload, err := dec.decodePES(VideoPES, dm)
	if err != nil {
		return nil, err
	}
	dec.decodeH264(payload)
	return t, nil
}

func (decoder *PsDecoder) decodePsHeader() (*ntree.NTree, error) {
	t := ntree.New(&Item{k: "pack header"})
	dm := NewDataManager(decoder.br, t)
	dm.decode(packHeader)
	//log.Printf("%+v offset:%d\n", dm.m, offset)
	// skip stuffing bytes
	dm.skipBytes(dm.getData("pack_stuffing_length"))
	return t, nil
}

func (dec *PsDecoder) writeH264FrameToFile(frame []byte) error {
	if _, err := dec.h264File.Write(frame); err != nil {
		log.Println(err)
		return err
	}
	// 可能是因为这个导致的写入变慢
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

func New(param *param.ConsoleParam, ch chan *reader.PktInfo) *PsDecoder {
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
