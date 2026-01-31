package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ReadMessage reads a JSON-RPC message from the reader (stdin).
// It handles the Content-Length header.
func ReadMessage(reader *bufio.Reader) ([]byte, error) {
	// 1. Read Headers
	length := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			// End of headers
			break
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			lenStr := strings.TrimPrefix(line, "Content-Length: ")
			length, err = strconv.Atoi(lenStr)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %v", err)
			}
		}
	}

	if length == 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}

	// 2. Read Body
	body := make([]byte, length)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// WriteMessage writes a JSON object to the writer (stdout) with headers.
func WriteMessage(writer io.Writer, msg interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Write Header
	fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(body))
	// Write Body
	_, err = writer.Write(body)
	return err
}
