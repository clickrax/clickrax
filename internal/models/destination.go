package models

import "strings"

const (
	DestPBS = "pbs"
	DestSMB = "smb"
	DestFTP = "ftp"
)

// BackupDestination is a backup target: PBS, SMB share, or FTP/FTPS server.
type BackupDestination struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // pbs, smb, ftp
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	// PBS
	URL         string `json:"url,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Datastore   string `json:"datastore,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	TokenID     string `json:"token_id,omitempty"`

	// SMB / FTP
	Host       string `json:"host,omitempty"`
	Port       int    `json:"port,omitempty"`
	RemotePath string `json:"remote_path,omitempty"`
	Domain     string `json:"domain,omitempty"`
	Username   string `json:"username,omitempty"`

	// SMB
	Share string `json:"share,omitempty"`

	// FTP
	TLS     bool `json:"tls,omitempty"`
	Passive bool `json:"passive,omitempty"`
}

func (d BackupDestination) NormalizedType() string {
	t := strings.ToLower(strings.TrimSpace(d.Type))
	if t == "" {
		return DestPBS
	}
	return t
}

func (d BackupDestination) IsPBS() bool  { return d.NormalizedType() == DestPBS }
func (d BackupDestination) IsSMB() bool  { return d.NormalizedType() == DestSMB }
func (d BackupDestination) IsFTP() bool  { return d.NormalizedType() == DestFTP }

func (d BackupDestination) ToPBSServer() PBSServer {
	return PBSServer{
		ID:          d.ID,
		Name:        d.Name,
		URL:         d.URL,
		Fingerprint: d.Fingerprint,
		Datastore:   d.Datastore,
		Namespace:   d.Namespace,
		TokenID:     d.TokenID,
		Description: d.Description,
	}
}

func PBSServerToDestination(s PBSServer) BackupDestination {
	return BackupDestination{
		ID:          s.ID,
		Type:        DestPBS,
		Name:        s.Name,
		URL:         s.URL,
		Fingerprint: s.Fingerprint,
		Datastore:   s.Datastore,
		Namespace:   s.Namespace,
		TokenID:     s.TokenID,
		Description: s.Description,
	}
}

func (j BackupJob) EffectiveDestinationID() string {
	if strings.TrimSpace(j.DestinationID) != "" {
		return j.DestinationID
	}
	return j.ServerID
}

func FindDestination(cfg *Config, id string) (*BackupDestination, bool) {
	for i := range cfg.Destinations {
		if cfg.Destinations[i].ID == id {
			cp := cfg.Destinations[i]
			return &cp, true
		}
	}
	for _, s := range cfg.Servers {
		if s.ID == id {
			d := PBSServerToDestination(s)
			return &d, true
		}
	}
	return nil, false
}
