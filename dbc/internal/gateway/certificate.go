package gateway

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync/atomic"
	"time"
)

// CertificateReloader 讓 cert-manager 更新掛載 Secret 後，新 TLS 連線不需重啟 Pod。
// Reload 失敗時保留最後一份有效憑證。
type CertificateReloader struct {
	certificateFile string
	privateKeyFile  string
	current         atomic.Pointer[tls.Certificate]
}

// NewCertificateReloader 載入初始憑證；初始憑證無效時 server 不可啟動。
func NewCertificateReloader(certificateFile, privateKeyFile string) (*CertificateReloader, error) {
	r := &CertificateReloader{certificateFile: certificateFile, privateKeyFile: privateKeyFile}
	if err := r.Reload(); err != nil {
		return nil, err
	}
	return r, nil
}

// Reload 原子替換憑證。
func (r *CertificateReloader) Reload() error {
	certificate, err := tls.LoadX509KeyPair(r.certificateFile, r.privateKeyFile)
	if err != nil {
		return fmt.Errorf("載入 TLS certificate/key 失敗: %w", err)
	}
	r.current.Store(&certificate)
	return nil
}

// TLSConfig 回傳強制 TLS 1.3、每次 handshake 取得目前憑證的設定。
func (r *CertificateReloader) TLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			certificate := r.current.Load()
			if certificate == nil {
				return nil, fmt.Errorf("TLS certificate 尚未載入")
			}
			return certificate, nil
		},
	}
}

// Watch 定期重新讀取 cert-manager 掛載的檔案。onError 只接收錯誤，不會清掉舊憑證。
func (r *CertificateReloader) Watch(ctx context.Context, interval time.Duration, onError func(error)) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Reload(); err != nil && onError != nil {
				onError(err)
			}
		}
	}
}
