package pbs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"pbs-win-backup/internal/i18n"
)

func (c *Client) doPOST(path string, form url.Values) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodPost, c.baseURL()+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", c.authHeader())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// StartSnapshotVerify runs PBS native verify for one snapshot (updates verification status in UI).
func (c *Client) StartSnapshotVerify(backupType, backupID string, backupTime int64) (string, error) {
	if backupType == "" {
		backupType = "host"
	}
	path := fmt.Sprintf("/api2/json/admin/datastore/%s/verify", url.PathEscape(c.server.Datastore))
	form := url.Values{}
	form.Set("backup-type", backupType)
	form.Set("backup-id", backupID)
	form.Set("backup-time", strconv.FormatInt(backupTime, 10))
	form.Set("ignore-verified", "false")
	if c.server.Namespace != "" {
		form.Set("ns", c.server.Namespace)
	}

	body, status, err := c.doPOST(path, form)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", apiError(status, body)
	}
	var result struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	upid := strings.TrimSpace(result.Data)
	if upid == "" {
		return "", i18n.E("pbs.verify.no_upid", nil)
	}
	return upid, nil
}

type TaskStatus struct {
	Status     string
	ExitStatus string
}

// TaskStatus polls PBS task state for a UPID returned by async API calls.
func (c *Client) TaskStatus(upid string) (TaskStatus, error) {
	encoded := url.PathEscape(upid)
	path := fmt.Sprintf("/api2/json/nodes/localhost/tasks/%s/status", encoded)
	body, status, err := c.doGET(path)
	if err != nil {
		return TaskStatus{}, err
	}
	if status != http.StatusOK {
		return TaskStatus{}, apiError(status, body)
	}
	var result struct {
		Data struct {
			Status     string `json:"status"`
			ExitStatus string `json:"exitstatus"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return TaskStatus{}, err
	}
	return TaskStatus{
		Status:     result.Data.Status,
		ExitStatus: result.Data.ExitStatus,
	}, nil
}

// WaitTask blocks until the PBS task finishes or ctx is cancelled.
func (c *Client) WaitTask(ctx context.Context, upid string) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		st, err := c.TaskStatus(upid)
		if err != nil {
			return err
		}
		switch strings.ToLower(st.Status) {
		case "stopped":
			exit := strings.TrimSpace(st.ExitStatus)
			if exit == "" || strings.EqualFold(exit, "OK") {
				return nil
			}
			return i18n.Ef("pbs.task_failed", map[string]string{"exit": exit})
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// WaitTaskDuration is a convenience wrapper with timeout.
func (c *Client) WaitTaskDuration(timeout time.Duration, upid string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.WaitTask(ctx, upid)
}
