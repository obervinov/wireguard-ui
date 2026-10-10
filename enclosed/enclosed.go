// Package enclosed shares client configs as one-time notes on an Enclosed
// instance (https://github.com/CorentinTh/enclosed).
//
// The note is encrypted here, the way the Enclosed web client does it, so the
// instance only ever stores ciphertext: the key travels in the URL fragment,
// which browsers do not send to the server.
package enclosed

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// Enclosed accepts TTLs between ten minutes and one month.
const (
	MinTTL = 10 * time.Minute
	MaxTTL = 30 * 24 * time.Hour
)

// Client creates notes on one Enclosed instance.
type Client struct {
	baseURL string
	ttl     time.Duration
	http    *http.Client
}

// File is attached to the note, so the viewer can download it as is.
type File struct {
	Name     string
	MimeType string
	Content  []byte
}

// Note is the result of a share: the link to hand over and when it expires.
type Note struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewClient returns nil when no instance is configured; a nil client reports
// itself as disabled.
func NewClient(baseURL string, ttl time.Duration) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil
	}
	if ttl < MinTTL {
		ttl = MinTTL
	}
	if ttl > MaxTTL {
		ttl = MaxTTL
	}
	return &Client{baseURL: baseURL, ttl: ttl, http: &http.Client{Timeout: 15 * time.Second}}
}

// Enabled is safe to call on a nil client.
func (c *Client) Enabled() bool {
	return c != nil
}

// Share stores content plus an optional file as a note that is deleted after
// the first read, and returns its link.
func (c *Client) Share(ctx context.Context, content string, file *File) (*Note, error) {
	payload, key, err := encryptNote(content, file)
	if err != nil {
		return nil, err
	}

	body, _ := json.Marshal(map[string]any{
		"payload":             payload,
		"ttlInSeconds":        int(c.ttl.Seconds()),
		"deleteAfterReading":  true,
		"encryptionAlgorithm": "aes-256-gcm",
		"serializationFormat": "cbor-array",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/notes", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create note: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("create note: %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	var created struct {
		NoteID string `json:"noteId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil || created.NoteID == "" {
		return nil, fmt.Errorf("create note: unexpected response")
	}

	// "dar" marks the note as deleted after reading, so the viewer warns before opening it
	return &Note{
		URL:       fmt.Sprintf("%s/%s#dar:%s", c.baseURL, created.NoteID, key),
		ExpiresAt: time.Now().Add(c.ttl).UTC(),
	}, nil
}

// encryptNote mirrors @enclosed/lib encryptNote without a password: a random
// 32-byte base key, PBKDF2-SHA256 (100k rounds, the base key as salt) for the
// AES-256-GCM key, and the note serialised as a CBOR array.
func encryptNote(content string, file *File) (payload, key string, err error) {
	baseKey := make([]byte, 32)
	if _, err := rand.Read(baseKey); err != nil {
		return "", "", err
	}
	masterKey := pbkdf2.Key(baseKey, baseKey, 100_000, 32, sha256.New)

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return "", "", err
	}
	// Seal appends the 16-byte tag, which is the layout Enclosed expects
	sealed := gcm.Seal(nil, iv, serializeNote(content, file), nil)

	b64 := base64.RawURLEncoding
	return b64.EncodeToString(iv) + ":" + b64.EncodeToString(sealed), b64.EncodeToString(baseKey), nil
}

// serializeNote writes [content, [[metadata, bytes], ...]], the "cbor-array"
// format, with the metadata of a file asset as the web client builds it.
func serializeNote(content string, file *File) []byte {
	var buf bytes.Buffer
	cborHead(&buf, 4, 2)
	cborText(&buf, content)
	if file == nil {
		cborHead(&buf, 4, 0)
		return buf.Bytes()
	}
	cborHead(&buf, 4, 1)
	cborHead(&buf, 4, 2)
	cborHead(&buf, 5, 4)
	cborText(&buf, "type")
	cborText(&buf, "file")
	cborText(&buf, "fileType")
	cborText(&buf, file.MimeType)
	cborText(&buf, "name")
	cborText(&buf, file.Name)
	cborText(&buf, "size")
	cborHead(&buf, 0, uint64(len(file.Content)))
	cborHead(&buf, 2, uint64(len(file.Content)))
	buf.Write(file.Content)
	return buf.Bytes()
}

func cborText(buf *bytes.Buffer, s string) {
	cborHead(buf, 3, uint64(len(s)))
	buf.WriteString(s)
}

// cborHead writes a major type with its argument in the shortest form.
func cborHead(buf *bytes.Buffer, major byte, n uint64) {
	m := major << 5
	switch {
	case n < 24:
		buf.WriteByte(m | byte(n))
	case n <= 0xff:
		buf.Write([]byte{m | 24, byte(n)})
	case n <= 0xffff:
		buf.WriteByte(m | 25)
		_ = binary.Write(buf, binary.BigEndian, uint16(n))
	case n <= 0xffffffff:
		buf.WriteByte(m | 26)
		_ = binary.Write(buf, binary.BigEndian, uint32(n))
	default:
		buf.WriteByte(m | 27)
		_ = binary.Write(buf, binary.BigEndian, n)
	}
}
