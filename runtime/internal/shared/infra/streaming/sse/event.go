package sse

import (
	"fmt"
	"io"
	"strings"
)

// Event is one Server-Sent Events frame.
type Event struct {
	ID   string
	Type string
	Data string
}

// WriteEvent writes a full SSE frame to w.
func WriteEvent(w io.Writer, e Event) error {
	if e.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", e.ID); err != nil {
			return err
		}
	}
	if e.Type != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", e.Type); err != nil {
			return err
		}
	}
	if err := WriteData(w, e.Data); err != nil {
		return err
	}
	_, err := fmt.Fprint(w, "\n")
	return err
}

// WriteData writes the data field, prefixing each line per the SSE spec.
func WriteData(w io.Writer, data string) error {
	data = strings.TrimRight(data, "\n")
	if data == "" {
		_, err := fmt.Fprint(w, "data:\n")
		return err
	}
	for _, line := range strings.Split(data, "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	return nil
}
