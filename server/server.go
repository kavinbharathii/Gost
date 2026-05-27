
package server

import (
	"bufio"
	"fmt"
	"net"
	"time"

	"github.com/kavinbharathii/gost/protocol"
	"github.com/kavinbharathii/gost/store"
)

type Server struct {
	store		*store.Store
	listener	net.Listener
}

func New(s *store.Store) *Server {
	return &Server{store: s}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", ":6379")

	if err != nil {
		return err
	}
	s.listener = ln

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("accept error:", err)
			continue
		}
		go s.handleConn(conn)
	}
}


func (s *Server) handleConn (conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		line := scanner.Text()
		cmd, err := protocol.Parse(line)

		if err != nil {
			conn.Write([]byte("-ERR " + err.Error() + "\n"))
			continue
		}


		switch cmd.Op {
		case "GET":
			val, ok := s.store.Get(cmd.Key)

			if !ok {
				conn.Write([]byte("$-1\n"))
			} else {
				conn.Write([]byte("+" + val + "\n"))
			}

		case "SET":
			ttl := time.Duration(0)
			if cmd.TTL > 0 {
				ttl = time.Duration(cmd.TTL) * time.Second
			}

			if err := s.store.Set(cmd.Key, cmd.Val, ttl); err != nil {
				conn.Write([]byte("-ERR " + err.Error() + "\n"))
				continue
			}
			conn.Write([]byte("+OK\n"))

		case "DEL":
			ok, err := s.store.Delete(cmd.Key)
			if err != nil {
				conn.Write([]byte("-ERR " + err.Error() + "\n"))
				continue
			}
			if !ok {
				conn.Write([]byte("$-1\n"))
			} else {
				conn.Write([]byte("+OK\n"))
			}
		}
	}
}

