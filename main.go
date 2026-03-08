package main

import (
	"log"

	"github.com/Yakitrak/notesmd-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
