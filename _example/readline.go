// +build ignore

package main

import (
	"log"

	"github.com/mattn/go-tty"
	"github.com/mattn/go-tty/ttyutil"
)

func main() {
	t, err := tty.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer t.Close()

	s, err := ttyutil.ReadLine(t)
	if err != nil {
		log.Fatal(err)
	}
	println(s)
}
