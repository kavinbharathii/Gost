package replication

import (
	"fmt"
	"bufio"
	"net"
	"os"
	"strconv"
	"strings"
)

type FollowerNode struct {
	leaderAddr	string
	walPath		string
	applyFn		func(op, key, value string, ttl int)
}

func NewFollower (leaderAddr, walPath string, applyFn func(op, key, value string, ttl int)) *FollowerNode {
	return &FollowerNode {
		leaderAddr: leaderAddr,
		walPath: 	walPath,
		applyFn:	applyFn,
	}
}

func (f *FollowerNode) currentOffset() int {
	file, err := os.Open(f.walPath)
	if err != nil {
		return 0
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}
	return count
}

func (f *FollowerNode) Start() error {
	conn, err := net.Dial("tcp", f.leaderAddr)
	if err != nil {
		return fmt.Errorf("could not connect to leader: %w", err)
	}

	offset := f.currentOffset()
	fmt.Fprintf(conn, "%d\n", offset)

	fmt.Printf("Connected to leader at %s (offset: %d)\n", f.leaderAddr, offset)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "SET":
			if len(parts) >= 3 {
				ttl := 0
				if len(parts) == 5 && parts[3] == "EX" {
					ttl, _ = strconv.Atoi(parts[4])
				}
				f.applyFn("SET", parts[1], parts[2], ttl)
			}
		case "DEL":
			if len(parts) >= 2 {
				f.applyFn("DEL", parts[1], "", 0)
			}
		}
	}

	return fmt.Errorf("lost connection to leader")
}



