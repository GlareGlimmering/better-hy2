package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"h-ui/model/bo"
)

func (h *Hysteria2Api) accountStatsRequest(method, path, secret string, body []byte, want int, result interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("http://127.0.0.1:%d%s", h.apiPort, path), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", secret)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("Traffic Stats %s unavailable: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != want {
		return fmt.Errorf("Traffic Stats %s returned HTTP %d", path, resp.StatusCode)
	}
	if result == nil {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (16<<20)+1))
	if err != nil || len(data) > 16<<20 || json.Unmarshal(data, result) != nil {
		return fmt.Errorf("invalid Traffic Stats response for %s", path)
	}
	return nil
}

func (h *Hysteria2Api) AccountConsistencyReady(secret string) error {
	var capabilities struct {
		AccountConsistency int `json:"accountConsistency"`
	}
	if err := h.accountStatsRequest(http.MethodGet, "/capabilities", secret, nil, 200, &capabilities); err != nil {
		return err
	}
	if capabilities.AccountConsistency != 2 {
		return fmt.Errorf("Traffic Stats account consistency protocol 2 required")
	}
	return nil
}

func (h *Hysteria2Api) TrafficBatch(secret string) (bo.Hysteria2TrafficBatch, error) {
	var batch bo.Hysteria2TrafficBatch
	err := h.accountStatsRequest(http.MethodGet, "/traffic/batch", secret, nil, 200, &batch)
	return batch, err
}

func (h *Hysteria2Api) AcknowledgeTrafficBatch(secret string, receipt bo.Hysteria2TrafficReceipt) error {
	body, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	return h.accountStatsRequest(http.MethodPost, "/traffic/ack", secret, body, 204, nil)
}

func (h *Hysteria2Api) PauseTraffic(secret string) error {
	return h.accountStatsRequest(http.MethodPost, "/traffic/pause", secret, nil, 204, nil)
}

func (h *Hysteria2Api) ResumeTraffic(secret string) error {
	return h.accountStatsRequest(http.MethodPost, "/traffic/resume", secret, nil, 204, nil)
}

func (h *Hysteria2Api) FenceAdmissions(secret string) error {
	return h.accountStatsRequest(http.MethodPost, "/admission/fence", secret, nil, 204, nil)
}
