package main

import (
	"log"
	"param"
	"parser"
	"reader"
	"ui"
)

const max int = 100

// TODO 待实现
// - mpeg ps
// - mpeg ts
// - flv
// - rtp over tcp
// - h264
// - h265
// - aac
// - mp4

func main() {
	log.SetFlags(log.Lshortfile)
	ch := make(chan *reader.PktInfo, max)
	param, err := param.ParseConsoleParam()
	if err != nil {
		return
	}
	parser := parser.New(param, ch)
	ui := ui.New(ch, parser)
	parser.Run()
	ui.Disp()
}
