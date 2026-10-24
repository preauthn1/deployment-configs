package proxy

import (
	"errors"
	"fmt"

	"example.invalid/go/logging"
)

// CloseAuditLogging flush 並關閉目前程序建立的 Cloud Logging clients。
// dbc-gateway 優雅下線時必須呼叫，避免 buffered query entries 遺失。
func CloseAuditLogging() error {
	loggingClientsMu.Lock()
	clients := make(map[string]*logging.Client, len(loggingClients))
	for project, client := range loggingClients {
		clients[project] = client
	}
	loggingClients = map[string]*logging.Client{}
	loggingClientsMu.Unlock()

	var errs []error
	for project, client := range clients {
		if err := client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("關閉專案 %s 的 Cloud Logging client 失敗: %w", project, err))
		}
	}
	return errors.Join(errs...)
}
