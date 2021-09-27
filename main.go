package main

import (
	"log"
	"parser"
	"ui"
)

const max int = 100

func main() {
	log.SetFlags(log.Lshortfile)
	ch := make(chan *ui.TableItem, max)
	ui := ui.New(ch)
	go parser.Process(ch)
	ui.Disp()
}
