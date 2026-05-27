
package store

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"strconv"
)

type Store struct {
	mu		sync.RWMutex
	data	map[string]string
	expiry	map[string]time.Time
	walFile	*os.File
}

func New(walPath string) (*Store, error) {
	f, err := os.OpenFile(walPath, os.O_APPEND | os.O_CREATE | os.O_RDWR, 0644)
	
	if err != nil {
		return nil, fmt.Errorf("could not open WAL: %w", err)
	}

	s := &Store {
		data: 		make(map[string]string),
		expiry:		make(map[string]time.Time),
		walFile: 	f,
	}

	if err := s.replay(); err != nil {
		return nil, fmt.Errorf("could not replay WAL: %w", err)
	}

	return s, nil
}

func (s *Store) replay() error {
	scanner := bufio.NewScanner(s.walFile)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)

		if len(parts) <= 0 {
			continue
		}

		switch parts[0] {
		case "SET":
			if len(parts) >= 3 {
				key := parts[1]
				val := parts[2]
				s.data[key] = val

				if len(parts) == 5 && parts[3] == "EX" {
					expiryUnix, err := strconv.ParseInt(parts[4], 10, 64)
					if err == nil {
						expiry := time.Unix(expiryUnix, 0)
						if time.Now().Before(expiry) {
							s.expiry[key] = expiry
						} else {
							// key already expired while server was down, skip it
							delete(s.data, key)
						}
					}
				}
			}
		case "DEL":
			if len(parts) >= 2 {
				delete(s.data, parts[1])
				delete(s.expiry, parts[1])
			}
		}
	}

	return scanner.Err()
}

func (s *Store) StartSweeper (interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			s.mu.Lock()
			now := time.Now()

			for key, exp := range s.expiry {
				if now.After(exp) {
					delete(s.data, key)
					delete(s.expiry, key)
				}
			}

			s.mu.Unlock()
		}
	}()
}

func (s *Store) log (entry string) error {
	_, err := fmt.Fprintln(s.walFile, entry)
	return err
}

func (s *Store) Get (key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	exp, hasExpiry := s.expiry[key]

	if hasExpiry && time.Now().After(exp) {
		delete(s.data, key)
		delete(s.expiry, key)
		return "", false
	}

	val, ok := s.data[key]
	return val, ok
}

func (s *Store) Set (key, value string, ttl time.Duration) error {
	entry := "SET " + key + " " + value

	if ttl > 0 {
		expiryUnix := time.Now().Add(ttl).Unix()
		entry += fmt.Sprintf(" EX %d", expiryUnix) 
	}

	if err := s.log(entry); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value

	if ttl > 0 {
		s.expiry[key] = time.Now().Add(ttl)
	} else {
		// case when the key had a ttl before
		// but since we passed 0, that entry has to be removed
		delete(s.expiry, key)
	}
	return nil
}

func (s *Store) Delete (key string) (bool, error) {

	if err := s.log("DEL " + key); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.data[key]

	if ok {
		delete(s.data, key)
		delete(s.expiry, key)
	}
	return ok, nil
}


