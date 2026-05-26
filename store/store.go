
package store

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

type Store struct {
	mu		sync.RWMutex
	data	map[string]string
	walFile	*os.File
}

func New(walPath string) (*Store, error) {
	f, err := os.OpenFile(walPath, os.O_APPEND | os.O_CREATE | os.O_RDWR, 0644)
	
	if err != nil {
		return nil, fmt.Errorf("could not open WAL: %w", err)
	}

	s := &Store {
		data: 		make(map[string]string),
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
				s.data[parts[1]] = parts[2]
			}
		case "DEL":
			if len(parts) >= 2 {
				delete(s.data, parts[1])
			}
		}
	}

	return scanner.Err()
}


func (s *Store) log (entry string) error {
	_, err := fmt.Fprintln(s.walFile, entry)
	return err
}

func (s *Store) Get (key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.data[key]
	return val, ok
}

func (s *Store) Set (key, value string) error {
	
	if err := s.log("SET " + key + " " + value); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
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
	}
	return ok, nil
}


