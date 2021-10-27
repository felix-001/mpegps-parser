package parser

import (
	"bitreader"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
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
	ch                 chan *ui.TableItem
}

func (decoder *PsDecoder) decodePkt(startCode uint32) (m map[string]interface{}, err error) {
	switch startCode {
	case StartCodePS:
		m, err = decoder.decodePsHeader()
	case StartCodeSYS:
		m, err = decoder.decodeSystemHeader()
	case StartCodeMAP:
		m, err = decoder.decodeProgramStreamMap()
	case StartCodeVideo:
		err = decoder.decodeVideoPes()
	case StartCodeAudio:
		err = decoder.decodeAudioPes()
	default:
		err = ErrNotFoundStartCode
	}
	return
}

func (decoder *PsDecoder) sendBasic(startCode uint32, m map[string]interface{}, status string) {
	if startCode == StartCodePS {
		return
	}
	offset, _ := decoder.br.Offset()
	item := &ui.TableItem{
		Offset:  int64(offset),
		PktType: m["pkt_type"].(string),
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
		m, err := decoder.decodePkt(uint32(startCode))
		status := "OK"
		if err != nil {
			status = "Error"
		}
		decoder.sendBasic(uint32(startCode), m, status)
		offset, _ = br.Offset()
	}
	return nil
}

func (dec *PsDecoder) decodeSystemHeader() (map[string]interface{}, error) {
	m := map[string]interface{}{
		"pkt_type":                     "system header",
		"header_length":                16,
		"marker_bit1":                  1,
		"rate_bound":                   22,
		"fixed_flag":                   1,
		"CSPS_flag":                    1,
		"system_audio_lock_flag":       1,
		"system_video_lock_flag":       1,
		"marker_bit2":                  1,
		"video_bound":                  5,
		"packet_rate_restriction_flag": 1,
		"reserved_bits":                7,
	}
	dec.decode(m)
	nextbits, err := dec.br.Peek(1)
	if err != nil {
		return nil, err
	}
	infos := []map[string]interface{}{}
	for nextbits == 1 {
		info := map[string]interface{}{
			"fixed":                    2,
			"P-STD_buffer_bound_scale": 1,
			"P-STD_buffer_size_bound":  13,
		}
		dec.decode(info)
		infos = append(infos, info)
		nextbits, err = dec.br.Peek(1)
		if err != nil {
			return nil, err
		}
	}
	m["nloop"] = infos
	return m, nil
}

func (dec *PsDecoder) decodeProgramStreamMap() (map[string]interface{}, error) {
	m := map[string]interface{}{
		"pkt_type":                   "program stream map",
		"map_stream_id":              8,
		"program_stream_map_length":  16,
		"current_next_indicator":     1,
		"reserved1":                  2,
		"program_stream_map_version": 5,
		"reserved2":                  7,
		"marker_bit":                 1,
		"program_stream_info_length": 16,
	}
	dec.decode(m)
	buf := make([]byte, m["program_stream_info_length"].(int))
	dec.br.ReadBytes(buf)
	elementary_stream_map_length, _ := dec.br.Read(16)
	m["elementary_stream_map_length"] = elementary_stream_map_length
	elementary_stream_maps := []map[string]interface{}{}
	for elementary_stream_map_length > 0 {
		elementary_stream_map := map[string]interface{}{
			"stream_type":                   8,
			"elementary_stream_id":          8,
			"elementary_stream_info_length": 16,
		}
		dec.decode(elementary_stream_map)
		elementary_stream_maps = append(elementary_stream_maps, elementary_stream_map)
		elementary_stream_info_length := elementary_stream_map["elementary_stream_info_length"].(uint64)
		buf := make([]byte, elementary_stream_info_length)
		dec.br.ReadBytes(buf)
		elementary_stream_map_length -= 4 + elementary_stream_info_length
	}
	m["elementary_stream_maps"] = elementary_stream_maps
	m["CRC_32"], _ = dec.br.Read(32)
	return m, nil
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

func (dec *PsDecoder) decodeAudioPes() error {
	dec.totalAudioFrameCnt++
	err := dec.decodePES(AudioPES)
	if err != nil {
		return err
	}
	payloadLen := dec.videoPes.PayloadLen
	payloadData := make([]byte, payloadLen)
	if _, err := dec.br.ReadBytes(payloadData); err != nil {
		log.Println(err)
		return err
	}
	dec.saveAudioPkt(payloadData, uint32(payloadLen), false)
	return nil
}

func (dec *PsDecoder) decodePTSDTS(m map[string]interface{}) error {
	PTS_DTS_flags := m["PTS_DTS_flags"].(uint64)
	if PTS_DTS_flags == 2 {
		ptsInfo := map[string]interface{}{
			"fixed":       4,
			"PTS1":        3,
			"marker_bit1": 1,
			"PTS2":        15,
			"marker_bit2": 1,
			"PTS3":        15,
			"marker_bit3": 1,
		}
		dec.decode(ptsInfo)
		m["PTS"] = ptsInfo
	}
	if PTS_DTS_flags == 3 {
		ptsdtsInfo := map[string]interface{}{
			"fixed1":      4,
			"PTS1":        3,
			"marker_bit1": 1,
			"PTS2":        15,
			"marker_bit2": 1,
			"PTS3":        15,
			"marker_bit3": 1,
			"fixed2":      4,
			"DTS1":        3,
			"marker_bit4": 1,
			"DTS2":        15,
			"marker_bit5": 1,
			"DTS3":        15,
			"marker_bit6": 1,
		}
		dec.decode(ptsdtsInfo)
		m["PTSDTS"] = ptsdtsInfo
	}
	return nil
}

func (dec *PsDecoder) decodeESCR(m map[string]interface{}) error {
	if m["ESCR_flag"].(uint64) != 1 {
		return nil
	}
	escrInfo := map[string]interface{}{
		"reserved":       1,
		"ESCR_base1":     3,
		"marker_bit1":    1,
		"ESCR_base2":     15,
		"marker_bit2":    1,
		"ESCR_base3":     15,
		"marker_bit3":    1,
		"ESCR_extension": 9,
		"marker_bit4":    1,
	}
	dec.decode(escrInfo)
	m["ESCR"] = escrInfo
	return nil
}

func (dec *PsDecoder) decodeESRate(m map[string]interface{}) error {
	if m["ES_rate_flag"].(uint64) != 1 {
		return nil
	}
	esRateInfo := map[string]interface{}{
		"marker_bit1": 1,
		"ES_rate":     22,
		"marker_bit2": 1,
	}
	dec.decode(esRateInfo)
	m["ES_rate"] = esRateInfo
	return nil
}

func (dec *PsDecoder) decodeDSMTrickMode(m map[string]interface{}) error {
	if m["(DSM_trick_mode_flag"].(uint64) != 1 {
		return nil
	}
	trick_mode_control, _ := dec.br.Read(3)
	m["trick_mode_control"] = trick_mode_control
	switch trick_mode_control {
	case fast_forward:
	case fast_reverse:
		info := map[string]interface{}{
			"field_id":             2,
			"intra_slice_refresh":  1,
			"frequency_truncation": 2,
		}
		dec.decode(info)
		m["trick_mode"] = info
	case slow_motion:
	case slow_reverse:
		m["rep_cntrl"], _ = dec.br.Read(5)
	default:
		dec.br.Read(5)

	}
	return nil
}

func (dec *PsDecoder) decodeAdditionalCopyInfo(m map[string]interface{}) error {
	if m["additional_copy_info_flag"].(uint64) == 1 {
		dec.br.Read(1)
		m["additional_copy_info"], _ = dec.br.Read(7)
	}
	return nil
}

func (dec *PsDecoder) decodeCRC(m map[string]interface{}) error {
	if m["PES_CRC_flag"].(uint64) == 1 {
		m["previous_PES_packet_CRC"], _ = dec.br.Read(16)
	}
	return nil
}

func (dec *PsDecoder) decodePesBase() (map[string]interface{}, error) {
	m := map[string]interface{}{
		"PES_packet_length":         16,
		"fixed":                     2,
		"PES_scrambling_control":    1,
		"PES_priority":              1,
		"data_alignment_indicator":  1,
		"copyright":                 1,
		"original_or_copy":          1,
		"PTS_DTS_flags":             2,
		"ESCR_flag":                 1,
		"ES_rate_flag":              1,
		"DSM_trick_mode_flag":       1,
		"additional_copy_info_flag": 1,
		"PES_CRC_flag":              1,
		"PES_extension_flag":        1,
		"PES_header_data_length":    8,
	}
	dec.decode(m)
	return m, nil
}

func (dec *PsDecoder) decodePesExtension(m map[string]interface{}) error {
	pesExt := map[string]interface{}{
		"PES_private_data_flag":                1,
		"pack_header_field_flag":               1,
		"program_packet_sequence_counter_flag": 1,
		"P-STD_buffer_flag":                    1,
		"reserved":                             3,
		"PES_extension_flag_2":                 1,
	}
	dec.decode(pesExt)
	m["PES_xtension"] = pesExt
	if m["PES_private_data_flag"].(uint64) == 1 {
		b := make([]byte, 16)
		dec.br.ReadBytes(b)
	}
	if m["pack_header_field_flag"].(uint64) == 1 {
		pack_field_length, _ := dec.br.Read(8)
		m["pack_field_length"] = pack_field_length
		b := make([]byte, pack_field_length)
		dec.br.ReadBytes(b)
	}
	if m["program_packet_sequence_counter_flag"].(uint64) == 1 {
		sequenceCount := map[string]interface{}{
			"marker_bit1":                     1,
			"program_packet_sequence_counter": 7,
			"marker_bit2":                     1,
			"MPEG1_MPEG2_identifier":          1,
			"original_stuff_length":           6,
		}
		dec.decode(sequenceCount)
		m["sequence_counter"] = sequenceCount
	}
	if m[" P-STD_buffer_flag"].(uint64) == 1 {
		dec.br.Read(2)
		m["P-STD_buffer_scale"], _ = dec.br.Read(1)
		m["P-STD_buffer_size"], _ = dec.br.Read(13)
	}
	if m["PES_extension_flag_2"].(uint64) == 1 {
		dec.br.Read(1)
		PES_extension_field_length, _ := dec.br.Read(7)
		m["PES_extension_field_length"] = PES_extension_field_length
		b := make([]byte, PES_extension_field_length)
		dec.br.ReadBytes(b)
	}
	return nil
}

func (dec *PsDecoder) decodePESHeader() error {
	m, _ := dec.decodePesBase()
	dec.decodePTSDTS(m)
	dec.decodeESCR(m)
	dec.decodeESRate(m)
	dec.decodeDSMTrickMode(m)
	dec.decodeAdditionalCopyInfo(m)
	dec.decodeCRC(m)

	br := dec.br
	/* payload length */
	payloadLen, err := br.Read(16)
	if err != nil {
		log.Println(err)
		return err
	}

	/* flags: pts_dts_flags ... */
	br.Read(16) // 跳过各种flags,比如pts_dts_flags
	payloadLen -= 2

	/* pes header data length */
	pesHeaderDataLen, err := br.Read(8)
	if err != nil {
		log.Println(err)
		return err
	}
	payloadLen--

	/* pes header data */
	b := make([]byte, pesHeaderDataLen)
	br.ReadBytes(b)
	payloadLen -= pesHeaderDataLen

	videoPes := dec.videoPes
	videoPes.PayloadLen = payloadLen
	videoPes.PesHeaderDataLen = pesHeaderDataLen

	return nil
}

func (dec *PsDecoder) decodePES(pesType int) error {
	offset, _ := dec.br.Offset()
	pesStartPos := offset - 4 // 4为startcode的长度
	err := dec.decodePESHeader()
	if err != nil {
		return err
	}
	payloadLen := uint32(dec.videoPes.PayloadLen)
	if !dec.isPayloadLenValid(payloadLen, pesType, pesStartPos) {
		dec.ReadInvalidBytes(payloadLen, pesType, pesStartPos)
		return ErrCheckPayloadLen
	}

	return nil
}

func (dec *PsDecoder) decodeVideoPes() error {
	dec.totalVideoFrameCnt++
	err := dec.decodePES(VideoPES)
	if err != nil {
		return err
	}
	payloadLen := dec.videoPes.PayloadLen
	payloadData := make([]byte, payloadLen)
	if _, err := dec.br.ReadBytes(payloadData); err != nil {
		log.Println(err)
		return err
	}
	dec.decodeH264(payloadData, uint32(payloadLen), false)
	return nil
}

func (decoder *PsDecoder) decode(m map[string]interface{}) error {
	for k, v := range m {
		_v, ok := v.(uint64)
		if !ok {
			continue
		}
		ret, err := decoder.br.Read(uint(_v))
		if err != nil {
			log.Println("read", k, v, "err")
			return err
		}
		m[k] = ret
	}
	return nil
}

func (decoder *PsDecoder) decodePsHeader() (map[string]interface{}, error) {
	m := map[string]interface{}{
		"pkt_type":                         "pack header",
		"fixed":                            2,
		"system_clock_refrence_base1":      3,
		"marker_bit1":                      1,
		"system_clock_refrence_base2":      15,
		"marker_bit2":                      1,
		"system_clock_refrence_base3":      15,
		"marker_bit3":                      1,
		"system_clock_reference_extension": 9,
		"marker_bit4":                      1,
		"program_mux_rate":                 22,
		"marker_bit5":                      1,
		"marker_bit6":                      1,
		"resvrved":                         5,
		"pack_stuffing_length":             3,
	}
	decoder.decode(m)
	// skip stuffing bytes
	decoder.br.Read(uint(m["pack_stuffing_length"].(uint64) * 8))
	return m, nil
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

func (decoder *PsDecoder) ParseDetail(offset int, typ string) error {
	switch typ {
	case "video pes":
		return decoder.decodeVideoPes()
	case "audio pes":
		return decoder.decodeAudioPes()
	case "program stream map":
		return decoder.decodeProgramStreamMap()
	}
	return nil
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
