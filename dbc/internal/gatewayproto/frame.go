// Package gatewayproto 定義 dbc client 與 dbc-gateway 在 MySQL 資料流開始前使用的認證協定。
package gatewayproto

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	// Version 是目前的 wire protocol 版本。
	Version = 1
	// MaxFrameSize 限制認證 frame，避免未驗證的連線造成無界記憶體配置。
	MaxFrameSize = 64 * 1024
)

// ClientAuth 是 TLS 建立後，client 傳給 gateway 的第一個 frame。
// Target 是 server-side allowlist 中的邏輯名稱，不能是任意 host:port。
type ClientAuth struct {
	Version       int    `json:"version"`
	IDToken       string `json:"idToken"`
	Target        string `json:"target"`
	ClientVersion string `json:"clientVersion"`
}

// AuthResult 是 gateway 在開始轉送 MySQL protocol 前的回覆。
type AuthResult struct {
	OK        bool   `json:"ok"`
	SessionID string `json:"sessionId,omitempty"`
	Error     string `json:"error,omitempty"`
}

// WriteFrame 寫入 4-byte big-endian 長度及 JSON payload。
func WriteFrame(w io.Writer, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("編碼 gateway frame 失敗: %w", err)
	}
	if len(payload) == 0 || len(payload) > MaxFrameSize {
		return fmt.Errorf("gateway frame 大小 %d 超出限制", len(payload))
	}

	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(payload)))
	if err := writeAll(w, header[:]); err != nil {
		return fmt.Errorf("寫入 gateway frame header 失敗: %w", err)
	}
	if err := writeAll(w, payload); err != nil {
		return fmt.Errorf("寫入 gateway frame payload 失敗: %w", err)
	}
	return nil
}

// ReadFrame 讀取一個有大小上限的 frame，並拒絕未知 JSON 欄位。
func ReadFrame(r io.Reader, value any) error {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return fmt.Errorf("讀取 gateway frame header 失敗: %w", err)
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > MaxFrameSize {
		return fmt.Errorf("gateway frame 大小 %d 超出限制", size)
	}

	payload := make([]byte, int(size))
	if _, err := io.ReadFull(r, payload); err != nil {
		return fmt.Errorf("讀取 gateway frame payload 失敗: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("解析 gateway frame 失敗: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("解析 gateway frame 失敗: payload 包含多餘資料")
	}
	return nil
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
