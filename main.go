
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/kavinbharathii/gost/server"
	"github.com/kavinbharathii/gost/store"
)

func main() {
	mode := flag.String("mode", "leader", "leader or follower")
	port := flag.String("port", "6379", "port to listen on")
	leaderAddr := flag.String("leader", "", "leader address (follower mode only")
	replPort := flag.String("repl-port", "6380", "replication port (leader mode only)")
	flag.Parse()

	walPath := fmt.Sprintf("gost-%s.wal", *port)
	s, err := store.New(walPath)
	if err != nil {
		log.Fatal(err)
	}

	s.StartSweeper(5 * time.Second)

	srv := server.New(s, *mode, walPath)

	fmt.Printf("Gost running in %s mode on :%s\n", *mode, *port)
	if err := srv.Start(":" + *port, ":" + *replPort, *leaderAddr); err != nil {
		log.Fatal(err)
	}
}




