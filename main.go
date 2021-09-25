package main

import (
	"gui"
	"log"
	"parser"
)

const max int = 100

func main() {
	log.SetFlags(log.Lshortfile)
	ch := make(chan *gui.TableItem, max)
	gui := gui.New(ch)
	go parser.Process(ch)
	gui.Disp()
}
