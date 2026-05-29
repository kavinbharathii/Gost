
package server

import (
	"bufio"
	"fmt"
	"net"
	"time"

	"github.com/kavinbharathii/gost/protocol"
	"github.com/kavinbharathii/gost/replication"
	"github.com/kavinbharathii/gost/store"
)

type Server struct {
	store		*store.Store
	listener	net.Listener
	mode		string
	leader		*replication.Leader
	walPath		string
}

func New(s *store.Store, mode string, walPath string) *Server {
	return &Server{store: s, mode: mode, walPath: walPath}
}

func (s *Server) Start(addr string, replPort string, leaderAddr string) error {
	if s.mode == "leader" {
		s.leader = replication.NewLeader(s.walPath)
		if err := s.leader.Start(replPort); err != nil {
			return err
		}
	}

	if s.mode == "follower" {
		follower := replication.NewFollower(leaderAddr, s.walPath, func (op, key, value string, ttl int){
			if op == "SET" {
				s.store.Set(key, value, time.Duration(ttl) * time.Second)
			} else if op == "DEL" {
				s.store.Delete(key)
			}
		})

		go func() {
			if err := follower.Start(); err != nil {
				fmt.Println("replication error:", err)
			}
		}()
	}

	ln, err := net.Listen("tcp", addr) 
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

		if (cmd.Op == "SET" || cmd.Op == "DEL") && s.mode == "follower" {
			conn.Write([]byte("-ERR this is a follower node, writes not allowed\n"))
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
			if s.leader != nil {
				entry := "SET " + cmd.Key + " " + cmd.Val
				if cmd.TTL > 0 {
					entry += fmt.Sprintf(" EX %d", cmd.TTL)
				}
				s.leader.Publish(entry)
			}
			conn.Write([]byte("+OK\n"))

		case "DEL":
			ok, err := s.store.Delete(cmd.Key)
			if err != nil {
				conn.Write([]byte("-ERR " + err.Error() + "\n"))
				continue
			}
			if s.leader != nil {
				s.leader.Publish("DEL " + cmd.Key)
			}
			if !ok {
				conn.Write([]byte("$-1\n"))
			} else {
				conn.Write([]byte("+OK\n"))
			}
		}
	}
}

