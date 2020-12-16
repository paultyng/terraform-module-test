package main

import (
	"log"
	"os"

	"github.com/paultyng/terraform-module-test/internal/cmd"
)

func main() {
	err := cmd.Run(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}
