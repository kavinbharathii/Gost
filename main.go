
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/kavinbharathii/gost/server"
	"github.com/kavinbharathii/gost/store"
)

func main() {
	s, err := store.New("gost.wal")
	if err != nil {
		log.Fatal(err)
	}

	srv := server.New(s)
	s.StartSweeper(5 * time.Second)

	fmt.Println("Gost listening on port :6379")
	if err := srv.Start(); err != nil {
		log.Fatal(err)
	}
}
