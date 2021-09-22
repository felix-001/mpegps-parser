package main

import (
	"gui"
	"parser"
)

const max int = 100

func main() {
	ch := make(chan string, max)
	gui := gui.New()
	go gui.ShowData(ch)
	go parser.Process(ch)
	gui.Disp()
}
