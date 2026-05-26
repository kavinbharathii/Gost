
package main

import (
	"fmt"
	"log"

	"github.com/kavinbharathii/gost/server"
	"github.com/kavinbharathii/gost/store"
)

func main() {
	s := store.New()
	srv := server.New(s)

	fmt.Println("Gost listening on port :6379")
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}
