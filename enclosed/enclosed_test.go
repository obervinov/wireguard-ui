package enclosed

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// decrypt follows the Enclosed node decryptString: iv:ciphertext+tag, key from the URL fragment.
func decrypt(t *testing.T, payload, key string) []byte {
	t.Helper()
	b64 := base64.RawURLEncoding
	parts := strings.Split(payload, ":")
	if len(parts) != 2 {
		t.Fatalf("payload %q is not iv:ciphertext", payload)
	}
	iv, _ := b64.DecodeString(parts[0])
	sealed, _ := b64.DecodeString(parts[1])
	baseKey, err := b64.DecodeString(key)
	if err != nil || len(baseKey) != 32 {
		t.Fatalf("bad key %q", key)
	}
	block, _ := aes.NewCipher(pbkdf2.Key(baseKey, baseKey, 100_000, 32, sha256.New))
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, iv, sealed, nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	return plain
}

func TestShareCreatesDecryptableOneTimeNote(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/notes" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"noteId":"abc123"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/", time.Minute) // below the minimum, clamped to 10m
	conf := "[Interface]\nPrivateKey = x\n"
	note, err := c.Share(context.Background(), conf, &File{Name: "phone.conf", MimeType: "text/plain", Content: []byte(conf)})
	if err != nil {
		t.Fatal(err)
	}

	if got["deleteAfterReading"] != true || got["ttlInSeconds"] != float64(600) ||
		got["encryptionAlgorithm"] != "aes-256-gcm" || got["serializationFormat"] != "cbor-array" {
		t.Errorf("unexpected request %v", got)
	}
	prefix := srv.URL + "/abc123#dar:"
	if !strings.HasPrefix(note.URL, prefix) {
		t.Fatalf("url %q, want prefix %q", note.URL, prefix)
	}
	key := strings.TrimPrefix(note.URL, prefix)
	if strings.Contains(got["payload"].(string), key) {
		t.Error("the key must not be sent to the server")
	}

	plain := decrypt(t, got["payload"].(string), key)
	if want := serializeNote(conf, &File{Name: "phone.conf", MimeType: "text/plain", Content: []byte(conf)}); string(plain) != string(want) {
		t.Errorf("decrypted note differs from the serialised one")
	}
}

func TestSerializeNoteCBOR(t *testing.T) {
	// ["hi", []]
	if got := serializeNote("hi", nil); string(got) != "\x82\x62hi\x80" {
		t.Errorf("got % x", got)
	}
	// a 300-byte text needs the two-byte length form
	long := strings.Repeat("a", 300)
	if got := serializeNote(long, nil); string(got[:4]) != "\x82\x79\x01\x2c" {
		t.Errorf("got % x", got[:4])
	}
	f := serializeNote("c", &File{Name: "a.conf", MimeType: "text/plain", Content: []byte("xy")})
	want := "\x82\x61c\x81\x82\xa4" +
		"\x64type\x64file" + "\x68fileType\x6atext/plain" + "\x64name\x66a.conf" + "\x64size\x02" +
		"\x42xy"
	if string(f) != want {
		t.Errorf("got  % x\nwant % x", f, want)
	}
}

func TestShareReportsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"payload too large"}`, http.StatusRequestEntityTooLarge)
	}))
	defer srv.Close()
	if _, err := NewClient(srv.URL, time.Hour).Share(context.Background(), "x", nil); err == nil || !strings.Contains(err.Error(), "413") {
		t.Errorf("want a 413 error, got %v", err)
	}
}

func TestDisabledWithoutURL(t *testing.T) {
	if NewClient("  ", time.Hour).Enabled() {
		t.Error("an empty URL must disable sharing")
	}
}
