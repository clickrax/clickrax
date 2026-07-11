package pbs

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"pbs-win-backup/internal/i18n"
	"pbs-win-backup/internal/i18nconfig"
	"pbs-win-backup/internal/models"
	"pbs-win-backup/internal/notify"
)

type Client struct {
	server     models.PBSServer
	secret     string
	httpClient *http.Client
}

func NewClient(server models.PBSServer, secret string) *Client {
	return &Client{
		server:     server,
		secret:     secret,
		httpClient: NewHTTPClient(server.Fingerprint, 120*time.Second),
	}
}

func normalizeFingerprint(fp string) string {
	fp = strings.ToLower(fp)
	fp = strings.ReplaceAll(fp, ":", "")
	fp = strings.ReplaceAll(fp, " ", "")
	return fp
}

func tlsConfig(fingerprint string) *tls.Config {
	fp := normalizeFingerprint(fingerprint)
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if fp == "" {
		return cfg
	}
	cfg.InsecureSkipVerify = true
	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		for _, cert := range cs.PeerCertificates {
			sum := sha256.Sum256(cert.Raw)
			certFP := hex.EncodeToString(sum[:])
			if certFP == fp {
				return nil
			}
		}
		return i18n.E("pbs.fingerprint_mismatch", nil)
	}
	return cfg
}

func (c *Client) authHeader() string {
	return fmt.Sprintf("PBSAPIToken=%s:%s", c.server.TokenID, c.secret)
}

func (c *Client) baseURL() string {
	return strings.TrimRight(c.server.URL, "/")
}

func datastoreSnapshotsPath(datastore, namespace string) string {
	path := fmt.Sprintf("/api2/json/admin/datastore/%s/snapshots", datastore)
	if namespace != "" {
		path += "?" + url.Values{"ns": {namespace}}.Encode()
	}
	return path
}

func (c *Client) doGET(path string) ([]byte, int, error) {
	return c.doGETWithContext(context.Background(), path)
}

func (c *Client) doGETWithContext(ctx context.Context, path string) ([]byte, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL()+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", c.authHeader())

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

func parseAPIErrorMessage(body []byte) string {
	var envelope struct {
		Errors map[string]json.RawMessage `json:"errors"`
		Data   json.RawMessage            `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Errors) > 0 {
		parts := make([]string, 0, len(envelope.Errors))
		for k, v := range envelope.Errors {
			msg := strings.Trim(string(v), `"`)
			if msg == "" {
				msg = string(v)
			}
			parts = append(parts, fmt.Sprintf("%s: %s", k, msg))
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	msg := strings.TrimSpace(string(body))
	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	return msg
}

func apiError(status int, body []byte) error {
	b := i18nconfig.FromConfig()
	switch status {
	case 401:
		return b.E("pbs.auth_invalid")
	case 403:
		return b.E("pbs.forbidden")
	case 0:
		return b.E("pbs.no_connection")
	default:
		if detail := parseAPIErrorMessage(body); detail != "" {
			return b.Ef("pbs.http_error", map[string]string{
				"code": fmt.Sprintf("%d", status), "detail": detail,
			})
		}
		return b.Ef("pbs.http_error_code", map[string]string{"code": fmt.Sprintf("%d", status)})
	}
}

func (c *Client) GetVersion() (string, error) {
	body, status, err := c.doGET("/api2/json/version")
	if err != nil {
		return "", i18n.Ewrap("pbs.no_connection_wrap", nil, err)
	}
	if status != http.StatusOK {
		return "", apiError(status, body)
	}

	var result struct {
		Data struct {
			Version string `json:"version"`
			Release string `json:"release"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	ver := result.Data.Version
	if ver == "" {
		ver = result.Data.Release
	}
	if ver == "" {
		ver = "unknown"
	}
	return ver, nil
}

func (c *Client) ListDatastores() ([]string, error) {
	body, status, err := c.doGET("/api2/json/admin/datastore")
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apiError(status, body)
	}

	var result struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(result.Data))
	for _, raw := range result.Data {
		var asString string
		if err := json.Unmarshal(raw, &asString); err == nil && asString != "" {
			names = append(names, asString)
			continue
		}
		var asObj struct {
			Name  string `json:"name"`
			Store string `json:"store"`
		}
		if err := json.Unmarshal(raw, &asObj); err == nil {
			if asObj.Store != "" {
				names = append(names, asObj.Store)
			} else if asObj.Name != "" {
				names = append(names, asObj.Name)
			}
		}
	}
	return names, nil
}

func (c *Client) testDatastoreAccess() error {
	if c.server.Datastore == "" {
		return nil
	}
	path := datastoreSnapshotsPath(c.server.Datastore, c.server.Namespace)
	body, status, err := c.doGET(path)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return apiError(status, body)
	}
	return nil
}

func (c *Client) TestConnection() models.ConnectionTestResult {
	b := i18nconfig.FromConfig()
	version, err := c.GetVersion()
	if err != nil {
		return models.ConnectionTestResult{
			OK:      false,
			Message: err.Error(),
		}
	}

	if err := c.testDatastoreAccess(); err != nil {
		return models.ConnectionTestResult{
			OK:         false,
			Message: b.Tf("dest.pbs_datastore_failed", map[string]string{
				"version": version,
				"err":     err.Error(),
			}),
			PBSVersion: version,
		}
	}

	datastores, dsErr := c.ListDatastores()
	if dsErr != nil {
		log.Printf("pbs: ListDatastores: %v", dsErr)
		ns := ""
		if c.server.Namespace != "" {
			ns = b.Tf("dest.online_pbs_ns", map[string]string{"ns": c.server.Namespace})
		}
		return models.ConnectionTestResult{
			OK:         true,
			Message: b.Tf("dest.online_pbs", map[string]string{
				"version":   version,
				"datastore": c.server.Datastore,
				"ns":        ns,
			}),
			PBSVersion: version,
		}
	}

	return models.ConnectionTestResult{
		OK: true,
		Message: b.Tf("dest.online_pbs_datastores", map[string]string{
			"version": version,
			"n":       fmt.Sprintf("%d", len(datastores)),
		}),
		PBSVersion: version,
		Datastores: datastores,
	}
}

func (c *Client) ListSnapshots() ([]models.SnapshotInfo, error) {
	return c.ListSnapshotsWithContext(context.Background())
}

func (c *Client) ListSnapshotsWithContext(ctx context.Context) ([]models.SnapshotInfo, error) {
	if c.server.Datastore == "" {
		return nil, i18n.E("pbs.datastore_missing", nil)
	}

	path := datastoreSnapshotsPath(c.server.Datastore, c.server.Namespace)

	body, status, err := c.doGETWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apiError(status, body)
	}

	var result struct {
		Data []struct {
			Backup     string `json:"backup"`
			BackupID   string `json:"backup-id"`
			BackupTime int64  `json:"backup-time"`
			Time       int64  `json:"time"`
			Comment    string `json:"comment"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	snaps := make([]models.SnapshotInfo, 0, len(result.Data))
	for _, s := range result.Data {
		backup := s.Backup
		if backup == "" {
			backup = s.BackupID
		}
		unix := s.BackupTime
		if unix == 0 {
			unix = s.Time
		}
		if unix == 0 {
			continue
		}
		snaps = append(snaps, models.SnapshotInfo{
			Backup:     backup,
			Time:       time.Unix(unix, 0).UTC().Format(time.RFC3339),
			BackupTime: unix,
			Comment:    s.Comment,
		})
	}
	return snaps, nil
}

// SnapshotManifestFiles lists archive filenames in a snapshot manifest.
func (c *Client) SnapshotManifestFiles(backupType, backupID string, backupTime int64) ([]string, error) {
	if c.server.Datastore == "" {
		return nil, i18n.E("pbs.datastore_missing", nil)
	}
	if backupType == "" {
		backupType = "host"
	}
	params := url.Values{}
	params.Set("backup-type", backupType)
	params.Set("backup-id", backupID)
	params.Set("backup-time", strconv.FormatInt(backupTime, 10))
	if c.server.Namespace != "" {
		params.Set("ns", c.server.Namespace)
	}
	path := fmt.Sprintf("/api2/json/admin/datastore/%s/files?%s",
		url.PathEscape(c.server.Datastore),
		params.Encode(),
	)

	body, status, err := c.doGET(path)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, apiError(status, body)
	}

	var result struct {
		Data []struct {
			Filename string `json:"filename"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(result.Data))
	for _, f := range result.Data {
		if f.Filename != "" {
			out = append(out, f.Filename)
		}
	}
	return out, nil
}

func (c *Client) SnapshotHasCatalog(backupType, backupID string, backupTime int64) (bool, error) {
	files, err := c.SnapshotManifestFiles(backupType, backupID, backupTime)
	if err != nil {
		return false, err
	}
	for _, name := range files {
		if name == "catalog.pcat1.didx" {
			return true, nil
		}
	}
	return false, nil
}

// FetchServerCertificateFingerprint connects without pinning and returns SHA-256 hex.
func FetchServerCertificateFingerprint(url string) (string, error) {
	if err := validateProbeURL(url); err != nil {
		return "", err
	}
	url = strings.TrimRight(url, "/")
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				VerifyConnection: func(cs tls.ConnectionState) error {
					return nil
				},
			},
		},
	}

	req, err := http.NewRequest(http.MethodGet, url+"/api2/json/version", nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", i18n.Ewrap("pbs.connect_failed", nil, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return "", i18n.E("pbs.no_tls_cert", nil)
	}
	sum := sha256.Sum256(resp.TLS.PeerCertificates[0].Raw)
	hexFP := hex.EncodeToString(sum[:])
	var parts []string
	for i := 0; i < len(hexFP); i += 2 {
		parts = append(parts, hexFP[i:i+2])
	}
	return strings.Join(parts, ":"), nil
}

func validateProbeURL(raw string) error {
	return notify.ValidateWebhookURL(raw)
}
