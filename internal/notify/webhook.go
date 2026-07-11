package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/models"
)

func SendWebhook(url string, result models.JobRunResult) error {
	if url == "" {
		return nil
	}
	if err := ValidateWebhookURL(url); err != nil {
		return err
	}
	body, err := json.Marshal(map[string]interface{}{
		"hostname":          machineHostname(),
		"job_name":          result.JobName,
		"status":            result.Status,
		"backup_type":       result.BackupType,
		"duration_sec":      result.DurationSec,
		"bytes_transferred": result.BytesTransferred,
		"bytes_reused":      result.BytesReused,
		"error":             result.Error,
		"snapshot":          result.Snapshot,
		"finished_at":       result.FinishedAt.Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := secureWebhookClient()
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook HTTP %d", resp.StatusCode)
	}
	return nil
}

func secureWebhookClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			if err := ValidateWebhookURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					host = addr
					port = "443"
				}
				if port == "" {
					port = "443"
				}
				ips, err := resolveWebhookIPs(host)
				if err != nil {
					return nil, err
				}
				var lastErr error
				for _, ip := range ips {
					target := net.JoinHostPort(ip.String(), port)
					conn, err := dialer.DialContext(ctx, network, target)
					if err == nil {
						return conn, nil
					}
					lastErr = err
				}
				if lastErr != nil {
					return nil, lastErr
				}
				return nil, errors.New("webhook dial failed")
			},
		},
	}
}

func resolveWebhookIPs(host string) ([]net.IP, error) {
	host = strings.Trim(host, "[]")
	if isBlockedWebhookHost(host) {
		return nil, i18nconfig.FromConfig().E("webhook.blocked_host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedWebhookIP(ip) {
			return nil, i18nconfig.FromConfig().E("webhook.blocked_host")
		}
		return []net.IP{ip}, nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, i18nconfig.FromConfig().Ef("webhook.invalid_url", map[string]string{"err": err.Error()})
	}
	if len(ips) == 0 {
		return nil, i18nconfig.FromConfig().E("webhook.no_host")
	}
	out := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		if isBlockedWebhookIP(ip) {
			return nil, i18nconfig.FromConfig().E("webhook.blocked_host")
		}
		out = append(out, ip)
	}
	return out, nil
}
