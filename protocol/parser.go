
package protocol

import (
	"errors"
	"strings"
)

type Command struct {
	Op	string
	Key	string
	Val	string
}

func Parse (line string) (Command, error) {
	parts := strings.Fields(line)

	if len(parts) <= 0 {
		return Command{}, errors.New("empty command")
	}

	op := strings.ToUpper(parts[0])
	
	switch op {
	case "GET":
		if len(parts) < 2 {
			return Command{}, errors.New("GET requires a key")
		}
		return Command{Op: op, Key: parts[1]}, nil
	
	case "SET":
		if len(parts) < 3 {
			return Command{}, errors.New("SET requires a key and value")
		}
		return Command{Op: op, Key: parts[1], Val: parts[2]}, nil
	
	case "DEL":
		if len(parts) < 2 {
			return Command{}, errors.New("DEL requires a key")
		}
		return Command{Op: op, Key: parts[1]}, nil
	
	default:
		return Command{}, errors.New("unknown command: " + op)
	}
}


