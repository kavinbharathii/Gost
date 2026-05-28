package replication

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync"
)


type Follower struct {
	conn	net.Conn
	send	chan string
}

type Leader struct {
	mu			sync.RWMutex
	followers	[]*Follower
	walPath		string
}

func NewLeader (walPath string) *Leader {
	return &Leader{
		walPath: walPath,
	}
}

func (l *Leader) Start (addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	fmt.Println("replication listener on", addr)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				fmt.Println("replication accept error:", err)
				continue
			}
			go l.handleFollower(conn)
		}
	}()

	return nil
}


func (l *Leader) handleFollower (conn net.Conn) {
	defer conn.Close()

	// read offset from follower
	scanner := bufio.NewScanner(conn)
	scanner.Scan()
	var offset int
	fmt.Sscanf(scanner.Text(), "%d", &offset)

	// stream WAL from offset
	f, err := os.Open(l.walPath)	
	if err != nil {
		fmt.Println("could not open WAL for replication:", err)
		return
	}

	defer f.Close()

	walScanner := bufio.NewScanner(f)
	current := 0
	for walScanner.Scan() {
		current++
		if current <= offset {
			continue
		}
		line := walScanner.Text()
		if line == "" {
			continue
		}

		fmt.Fprintf(conn, "%s\n", line)
	}

	// register follower for real-time updates
	follower := &Follower{
		conn: conn,
		send: make(chan string, 100),
	}

	l.mu.Lock()
	l.followers = append(l.followers, follower)
	l.mu.Unlock()

	// stream real time entries
	for entry := range follower.send {
		_, err := fmt.Fprintf(conn, "%s\n", entry)
		if err != nil {
			l.removeFollower(follower)
			return
		}
	}
}

func (l *Leader) Publish(entry string) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, f := range l.followers {
		select {
		case f.send <- entry:
		default:
			// follower is too slow, skip
		}
	}
}

func (l *Leader) removeFollower (follower *Follower) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for i, f := range l.followers {
		if f == follower {
			l.followers = append(l.followers[:i], l.followers[i+1:]...)
			return
		}
	}
}













