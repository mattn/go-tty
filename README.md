# go-tty

Simple tty utility

## Usage

```go
tty := tty.New()
defer tty.Close()

for {
	r, err := tty.ReadRune()
	if err != nil {
		log.Fatal(err)
	}
	// handle key event
}
```

## Installation

```
$ go get github.com/mattn/go-tty
```

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a mattn)
