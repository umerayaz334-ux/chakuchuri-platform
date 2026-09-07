package websocket

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	opText  = 0x1
	opClose = 0x8
	opPing  = 0x9
	opPong  = 0xa

	maxFrameBytes = 128 << 10
)

var ErrClosed = errors.New("websocket closed")

type Conn struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
}

func Accept(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if r.Method != http.MethodGet {
		return nil, errors.New("websocket requires GET")
	}
	if !headerContains(r.Header.Get("Connection"), "upgrade") || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, errors.New("missing websocket upgrade headers")
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		return nil, errors.New("unsupported websocket version")
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		return nil, errors.New("missing websocket key")
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("response writer cannot be hijacked")
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}
	// HTTP server read/write deadlines must not leak into this long-lived socket.
	if err := conn.SetDeadline(time.Time{}); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("clear websocket deadline: %w", err)
	}
	accept := acceptKey(key)
	response := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept)
	if _, err := conn.Write([]byte(response)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Conn{conn: conn, reader: rw.Reader}, nil
}

func (c *Conn) ReadJSON(target interface{}) error {
	payload, err := c.readMessage()
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, target)
}

func (c *Conn) WriteJSON(value interface{}) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.writeFrame(opText, payload)
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) readMessage() ([]byte, error) {
	for {
		opcode, payload, err := c.readFrame()
		if err != nil {
			return nil, err
		}
		switch opcode {
		case opText:
			return payload, nil
		case opPing:
			_ = c.writeFrame(opPong, payload)
		case opClose:
			_ = c.writeFrame(opClose, []byte{})
			return nil, ErrClosed
		}
	}
}

func (c *Conn) readFrame() (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, header); err != nil {
		return 0, nil, err
	}

	opcode := header[0] & 0x0f
	masked := header[1]&0x80 != 0
	length := uint64(header[1] & 0x7f)
	switch length {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(c.reader, extended); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(extended))
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(c.reader, extended); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(extended)
	}
	if length > maxFrameBytes {
		return 0, nil, errors.New("websocket frame too large")
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.reader, mask[:]); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for index := range payload {
			payload[index] ^= mask[index%4]
		}
	}
	return opcode, payload, nil
}

func (c *Conn) writeFrame(opcode byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	header := []byte{0x80 | opcode}
	length := len(payload)
	switch {
	case length <= 125:
		header = append(header, byte(length))
	case length <= 65535:
		header = append(header, 126, 0, 0)
		binary.BigEndian.PutUint16(header[len(header)-2:], uint16(length))
	default:
		header = append(header, 127, 0, 0, 0, 0, 0, 0, 0, 0)
		binary.BigEndian.PutUint64(header[len(header)-8:], uint64(length))
	}
	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := c.conn.Write(payload)
	return err
}

func acceptKey(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func headerContains(header string, value string) bool {
	for _, item := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return true
		}
	}
	return false
}
