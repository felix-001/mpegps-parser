package param

import (
	"errors"
	"flag"
	"log"
)

var (
	ErrCheckInputFile = errors.New("check input file error")
)

type ConsoleParam struct {
	PsFile            string
	OutputAudioFile   string
	OutputVideoFile   string
	DumpAudio         bool
	DumpVideo         bool
	PrintPsHeader     bool
	PrintSysHeader    bool
	PrintPsm          bool
	Verbose           bool
	DumpPesStartBytes bool
}

func ParseConsoleParam() (*ConsoleParam, error) {
	param := &ConsoleParam{}
	flag.StringVar(&param.PsFile, "file", "", "input file")
	flag.StringVar(&param.OutputAudioFile, "output-audio", "./output.audio", "output audio file")
	flag.StringVar(&param.OutputVideoFile, "output-video", "./output.video", "output video file")
	flag.BoolVar(&param.DumpAudio, "dump-audio", false, "dump audio")
	flag.BoolVar(&param.DumpVideo, "dump-video", false, "dump video")
	flag.BoolVar(&param.PrintPsHeader, "print-ps-header", false, "print ps header")
	flag.BoolVar(&param.PrintSysHeader, "print-sys-header", false, "print system header")
	flag.BoolVar(&param.PrintPsm, "print-psm", false, "print porgram stream map")
	flag.BoolVar(&param.Verbose, "verbose", false, "show packet detail")
	flag.BoolVar(&param.DumpPesStartBytes, "dump-pes-start-bytes", false, "dump pes start bytes")
	flag.Parse()
	if param.PsFile == "" {
		log.Println("must input file")
		return nil, ErrCheckInputFile
	}
	return param, nil
}
