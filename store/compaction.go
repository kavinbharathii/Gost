package store

import (
	"fmt"
	"os"
	"time"	
	"bufio"
	"strings"
	"strconv"
)

func (s *Store) Compact (walPath string) error {
	// count current wal lines
	walOffset, err := countLines(walPath)
	if err != nil {
		return fmt.Errorf("could not count WAL lines: %w", err)
	}

	tempData, tempExpiry, err := replayUpTo(walPath, walOffset)
	if err != nil {
		return fmt.Errorf("could not replay WAL: %w", err)
	}


	tmpPath := walPath + ".compact"
	tmp, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("could not create compact file: %w", err)
	}

	now := time.Now()
	for key, val := range tempData {
		exp, hasExpiry := tempExpiry[key]
		if hasExpiry && now.After(exp) {
			continue
		}

		if hasExpiry {
			fmt.Fprintf(tmp, "SET %s %s EX %d\n", key, val, exp.Unix())
		} else {
			fmt.Fprintf(tmp, "SET %s %s\n", key, val)
		}
	}

	tmp.Close()

	s.mu.Lock()
	defer s.mu.Unlock()

	tmp, err = os.OpenFile(tmpPath, os.O_APPEND | os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("could not reopen compact file:%w", err)
	}

	newEntries, err := readLinesFrom(walPath, walOffset)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("could not read new WAL entries: %w", err)
	}
	for _, line := range newEntries {
		fmt.Fprintf(tmp, "%s\n", line)
	}
	tmp.Close()

	if err := os.Rename(tmpPath, walPath); err != nil {
		return fmt.Errorf("could not rename compact file: %w", err)
	}

	f, err := os.OpenFile(walPath, os.O_APPEND | os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("could not reopen WAL after compaction: %w", err)
	}
	s.walFile.Close()
	s.walFile = f

	return nil
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}


func replayUpTo (path string, limit int) (map[string]string, map[string]time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	data := make(map[string]string)
	expiry := make(map[string]time.Time)

	scanner := bufio.NewScanner(f)
	current := 0
	for scanner.Scan() {
		current++
		if current> limit {
			break
		}

		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "SET":
			if len(parts) >= 3 {
				data[parts[1]] = parts[2]
				if len(parts) == 5 && parts[3] == "EX" {
					unix, err := strconv.ParseInt(parts[4], 10, 64)
					if err == nil {
						expiry[parts[1]] = time.Unix(unix, 0)
					}
				}
			}
		case "DEL":
			if len(parts) >= 2 {
				delete(data, parts[1])
				delete(expiry, parts[1])
			}
		}
	}

	return data, expiry, scanner.Err()
}

func readLinesFrom(path string, offset int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	current := 0
	for scanner.Scan() {
		current++
		if current <= offset {
			continue
		}
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}
