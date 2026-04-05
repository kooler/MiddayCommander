package audit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
)

// ReadRecent returns up to limit recent audit events, newest first.
func ReadRecent(path string, limit int) ([]Event, error) {
	if path == "" {
		return nil, os.ErrInvalid
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}

	if limit <= 0 {
		limit = 50
	}

	lines := make([][]byte, 0, limit)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		lines = append(lines, append([]byte(nil), line...))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	events := make([]Event, 0, min(limit, len(lines)))
	for i := len(lines) - 1; i >= 0 && len(events) < limit; i-- {
		var event Event
		if err := json.Unmarshal(lines[i], &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
