package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/32bitkid/bitreader"
)

const (
	UDPTransfer        int = 0
	TCPTransferActive  int = 1
	TCPTransferPassive int = 2
	LocalCache         int = 3
)

const (
	StreamTypeH264 = 0x1b
	StreamTypeH265 = 0x24
	StreamTypeAAC  = 0x90
)

const (
	StreamIDVideo = 0xe0
	StreamIDAudio = 0xc0
)

const (
	StartCodePS        = 0x000001ba
	StartCodeSYS       = 0x000001bb
	StartCodeMAP       = 0x000001bc
	StartCodeVideo     = 0x000001e0
	StartCodeAudio     = 0x000001c0
	MEPGProgramEndCode = 0x000001b9
)

const (
	MAXFrameLen int = 1024 * 1024 * 10
)

var (
	ErrNotFoundStartCode = errors.New("not found the need start code flag")
	ErrMarkerBit         = errors.New("marker bit value error")
	ErrFormatPack        = errors.New("not package standard")
	ErrParsePakcet       = errors.New("parse ps packet error")
	ErrParseRtp          = errors.New("parse rtp packet error")
)

type PsDecoder struct {
	rawData         []byte
	rawLen          int
	videoStreamType uint32
	audioStreamType uint32
	br              bitreader.BitReader
	psHeader        map[string]uint32
}

func (dec *PsDecoder) reSet() {
	dec.rawLen = 0
}

func (dec *PsDecoder) decPs() ([]byte, error) {

	br := dec.br
	for {
		startcode, err := br.Read32(32)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		switch startcode {
		case StartCodePS:
			log.Println("=== ps header ===")
			if err := dec.decodePsHeader(); err != nil {
				log.Println(err)
				return nil, err
			}
		case StartCodeSYS:
			log.Println("=== ps system header === ")
			if err := dec.decSystemHeader(); err != nil {
				log.Println(err)
				return nil, err
			}
		case StartCodeMAP:
			log.Println("=== program stream map ===")
			if err := dec.decProgramStreamMap(); err != nil {
				return nil, err
			}
		case StartCodeVideo:
			log.Println("=== video ===")
			if err := dec.decPESPacket(); err != nil {
				log.Println(err)
				return nil, err
			}
		case StartCodeAudio:
			log.Println("=== audio ===")
			raw := dec.rawData[:dec.rawLen]
			dec.rawLen = 0
			return raw, nil
		case MEPGProgramEndCode:
			log.Println("=== end ===")
			raw := dec.rawData[:dec.rawLen]
			dec.rawLen = 0
			return raw, nil
		default:
			log.Printf("parse start code error: 0x%x", startcode)
			dec.rawLen = 0
			return nil, ErrParsePakcet
		}
	}
}

func (dec *PsDecoder) decSystemHeader() error {
	br := dec.br
	syslens, err := br.Read32(16)
	log.Printf("\tsystem_header_length:%d", syslens)
	if err != nil {
		return err
	}

	br.Skip(uint(syslens) * 8)
	return nil
}

func (dec *PsDecoder) decProgramStreamMap() error {
	br := dec.br
	psmLen, err := br.Read32(16)
	if err != nil {
		return err
	}
	//log.Printf("\tprogram_stream_map_length: %d pos: %d", psmLen, br.Size() - int64(br.Len()))
	//drop psm version infor
	br.Skip(16)
	psmLen -= 2
	if programStreamInfoLen, err := br.Read32(16); err != nil {
		return err
	} else {
		br.Skip(uint(programStreamInfoLen * 8))
		psmLen -= (programStreamInfoLen + 2)
	}
	programStreamMapLen, err := br.Read32(16)
	if err != nil {
		return err
	}
	psmLen -= (2 + programStreamMapLen)
	log.Printf("\tprogram_stream_info_length: %d", programStreamMapLen)
	for programStreamMapLen > 0 {
		streamType, err := br.Read32(8)
		log.Printf("\t\tstream type: 0x%x", streamType)
		if err != nil {
			return err
		}
		elementaryStreamID, err := br.Read32(8)
		if err != nil {
			return err
		} else if elementaryStreamID >= 0xe0 && elementaryStreamID <= 0xef {
			dec.videoStreamType = streamType
		} else if elementaryStreamID >= 0xc0 && elementaryStreamID <= 0xdf {
			dec.audioStreamType = streamType
		}
		log.Printf("\t\tstream id: 0x%x", elementaryStreamID)
		if elementaryStreamInfoLength, err := br.Read32(16); err != nil {
			return err
		} else {
			log.Printf("\t\telementary_stream_info_length: %d", elementaryStreamInfoLength)
			br.Skip(uint(elementaryStreamInfoLength * 8))
			programStreamMapLen -= (4 + elementaryStreamInfoLength)
		}

	}

	// crc 32
	if psmLen != 4 {
		log.Printf("psmLen: 0x%x", psmLen)
		return ErrFormatPack
	}
	br.Skip(32)
	return nil
}

func (dec *PsDecoder) decodeH264(data []byte) error {
	if data[4] == 0x67 {
		log.Println("\t\tSPS")
	}
	if data[4] == 0x68 {
		log.Println("\t\tPPS")
	}
	if data[4] == 0x65 {
		log.Println("\t\tIDR")
	}
	return nil
}

func (dec *PsDecoder) decPESPacket() error {
	br := dec.br
	payloadlen, err := br.Read32(16)
	if err != nil {
		return err
	}
	log.Printf("\tPES_packet_length: %d", payloadlen)
	br.Skip(16)

	payloadlen -= 2
	if pesHeaderDataLen, err := br.Read32(8); err != nil {
		return err
	} else {
		log.Printf("\tpes_header_data_length: %d", pesHeaderDataLen)
		payloadlen--
		br.Skip(uint(pesHeaderDataLen * 8))
		payloadlen -= pesHeaderDataLen
	}

	payloaddata := make([]byte, payloadlen)
	if _, err := io.ReadAtLeast(br, payloaddata, int(payloadlen)); err != nil {
		return err
	} else {
		copy(dec.rawData[dec.rawLen:], payloaddata)
		dec.rawLen += int(payloadlen)
	}
	dec.decodeH264(payloaddata)

	return nil
}

type FieldInfo struct {
	len  uint
	item string
}

var psheaderFields = []FieldInfo{
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

func (decoder *PsDecoder) decodePsHeader() error {
	for _, field := range psheaderFields {
		val, err := decoder.br.Read32(field.len)
		if err != nil {
			log.Printf("parse %s error", field.item)
			return err
		}
		decoder.psHeader[field.item] = val
	}
	pack_stuffing_length := decoder.psHeader["pack_stuffing_length"]
	decoder.br.Skip(uint(pack_stuffing_length * 8))
	b, err := json.MarshalIndent(decoder.psHeader, "", "  ")
	if err != nil {
		log.Println("error:", err)
	}
	fmt.Print(string(b) + "\n")
	return nil
}

func main() {
	log.SetFlags(log.Lshortfile)
	ps_pkt, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Printf("open file: %s error", os.Args[1])
		return
	}
	ps_pkt = append(ps_pkt, 0x00, 0x00, 0x01, 0xb9)
	br := bitreader.NewReader(bytes.NewReader(ps_pkt))
	psDec := &PsDecoder{
		rawData:  make([]byte, MAXFrameLen),
		rawLen:   0,
		br:       br,
		psHeader: make(map[string]uint32),
	}
	psDec.reSet()
	h264, err := psDec.decPs()
	if err != nil {
		return
	}
	log.Printf("h264 len:%d", len(h264))
}
