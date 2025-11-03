package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

// MaxMessageSize defines the maximum size of a message
const MaxMessageSize = 1 << 16 // 64KB

// MessageType defines the type of message
type MessageType string

const (
	MsgTypeChallenge MessageType = "challenge"
	MsgTypeProof     MessageType = "proof"
	MsgTypeQuote     MessageType = "quote"
	MsgTypeError     MessageType = "error"
)

// BaseMessage for all messages
type BaseMessage struct {
	Type MessageType `json:"type"`
}

// ChallengeMessage is sent by the server
type ChallengeMessage struct {
	BaseMessage
	Challenge  string `json:"challenge"`  // Random string + timestamp
	Difficulty int    `json:"difficulty"` // Number of leading zeros in hash
}

// ProofMessage is sent by the client
type ProofMessage struct {
	BaseMessage
	Challenge string `json:"challenge"` // Echo the received challenge
	Nonce     string `json:"nonce"`     // Found nonce
}

// QuoteMessage is sent by the server
type QuoteMessage struct {
	BaseMessage
	Quote string `json:"quote"`
}

// ErrorMessage for errors
type ErrorMessage struct {
	BaseMessage
	Message string `json:"message"`
}

// WriteMessage writes a message to net.Conn with length prefix
func WriteMessage(conn net.Conn, msg interface{}, timeout time.Duration) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if len(jsonData) > MaxMessageSize {
		return fmt.Errorf("message size (%d) exceeds max allowed (%d)", len(jsonData), MaxMessageSize)
	}

	length := uint32(len(jsonData))
	lenBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(lenBuf, length)

	// Set write deadline
	if timeout > 0 {
		if err := conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
			return fmt.Errorf("failed to set write deadline: %w", err)
		}
		defer conn.SetWriteDeadline(time.Time{}) // Reset deadline
	}

	// Write length prefix - ensure all bytes are written
	if err := writeAll(conn, lenBuf); err != nil {
		return fmt.Errorf("failed to write message length: %w", err)
	}

	// Write message data - ensure all bytes are written
	if err := writeAll(conn, jsonData); err != nil {
		return fmt.Errorf("failed to write message data: %w", err)
	}

	return nil
}

// writeAll writes all data to conn, handling partial writes
func writeAll(conn net.Conn, data []byte) error {
	written := 0
	for written < len(data) {
		n, err := conn.Write(data[written:])
		if err != nil {
			return err
		}
		written += n
	}
	return nil
}

// ReadMessage reads a message from net.Conn with length prefix
func ReadMessage(conn net.Conn, target interface{}, timeout time.Duration) error {
	lenBuf := make([]byte, 4)

	// Set read deadline
	if timeout > 0 {
		if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return fmt.Errorf("failed to set read deadline: %w", err)
		}
		defer conn.SetReadDeadline(time.Time{}) // Reset deadline
	}

	// Read length prefix
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return fmt.Errorf("failed to read message length: %w", err)
	}

	length := binary.LittleEndian.Uint32(lenBuf)
	if length == 0 || length > MaxMessageSize {
		return fmt.Errorf("invalid message length: %d", length)
	}

	// Read message data
	msgBuf := make([]byte, length)
	if _, err := io.ReadFull(conn, msgBuf); err != nil {
		return fmt.Errorf("failed to read message data: %w", err)
	}

	// Unmarshal message
	if err := json.Unmarshal(msgBuf, target); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return nil
}
