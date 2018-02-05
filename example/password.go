package main

import (
	"fmt"

	"github.com/mattn/go-tty"
	//"os"
	//"os/signal"
)

func main() {
	tty, err := tty.Open()
	defer tty.Close()

	fmt.Print("Password: ")
	s, err := tty.ReadPassword()
	if err != nil {
		println("canceled")
		return
	}
	fmt.Println(s)
}
