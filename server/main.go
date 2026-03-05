package main

import (
	_ "embed"
	"fmt"
	"log"
)

//go:embed banner.txt
var banner string

func main() {
	fmt.Println(banner)

	app, err := NewApp()
	if err != nil {
		log.Fatalf("init: %v", err)
	}
	defer app.db.Close()

	if err := app.Run(); err != nil {
		log.Fatalf("run: %v", err)
	}
}
