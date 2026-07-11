package notify

import (
	"crypto/tls"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"pbs-win-backup/internal/i18nconfig"
)

const (
	smtpConnectTimeout = 30 * time.Second
	smtpIOTimeout      = 60 * time.Second
)

type unencryptedAuth struct {
	smtp.Auth
}

func (a unencryptedAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	s := *server
	s.TLS = true
	return a.Auth.Start(&s)
}

func smtpPort(port int) string {
	if port <= 0 {
		return "587"
	}
	return strconv.Itoa(port)
}

func tlsConfigFor(host string, insecure bool) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: insecure,
		ServerName:         host,
	}
}

func setConnDeadline(conn net.Conn) {
	if conn != nil {
		_ = conn.SetDeadline(time.Now().Add(smtpIOTimeout))
	}
}

func dialPlainSMTP(addr, host string) (*smtp.Client, net.Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, smtpConnectTimeout)
	if err != nil {
		return nil, nil, err
	}
	setConnDeadline(conn)
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return client, conn, nil
}

func dialImplicitTLS(addr, host string, tlsConfig *tls.Config) (*smtp.Client, net.Conn, error) {
	dialer := &net.Dialer{Timeout: smtpConnectTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	if err != nil {
		return nil, nil, err
	}
	setConnDeadline(conn)
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return client, conn, nil
}

func tryStartTLS(client *smtp.Client, tlsConfig *tls.Config) (bool, error) {
	if tlsConfig == nil {
		return false, nil
	}
	ok, _ := client.Extension("STARTTLS")
	if !ok {
		return false, nil
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return false, err
	}
	return true, nil
}

func dialSMTP(host string, port int, username, password string, insecure bool) (*smtp.Client, error) {
	portStr := smtpPort(port)
	addr := host + ":" + portStr
	tlsConfig := tlsConfigFor(host, insecure)

	var client *smtp.Client
	var conn net.Conn
	var err error
	tlsActive := false

	if portStr == "465" {
		client, conn, err = dialImplicitTLS(addr, host, tlsConfig)
		if err != nil {
			return nil, err
		}
		tlsActive = true
	} else {
		client, conn, err = dialPlainSMTP(addr, host)
		if err != nil {
			return nil, err
		}
		started, startErr := tryStartTLS(client, tlsConfig)
		if startErr != nil {
			if portStr == "25" && insecure {
				// fall through — cleartext on port 25 only when explicitly allowed
			} else {
				_ = client.Close()
				return nil, startErr
			}
		} else if started {
			tlsActive = true
			setConnDeadline(conn)
		}
	}

	auth, err := smtpAuth(host, username, password, tlsActive, portStr == "25" && insecure)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	if auth != nil {
		setConnDeadline(conn)
		if err := client.Auth(auth); err != nil {
			_ = client.Close()
			return nil, err
		}
	}
	return client, nil
}

func smtpAuth(host, username, password string, tlsActive bool, allowCleartext bool) (smtp.Auth, error) {
	if username == "" && password == "" {
		return nil, nil
	}
	auth := smtp.PlainAuth("", username, password, host)
	if tlsActive {
		return auth, nil
	}
	if allowCleartext {
		return unencryptedAuth{auth}, nil
	}
	return nil, i18nconfig.FromConfig().E("smtp.need_encryption")
}

func parseRecipients(to string) []string {
	parts := strings.Split(to, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func sendSMTPMessage(host string, port int, username, password, from, to, subject, body string, insecure bool) error {
	recipients := parseRecipients(to)
	if len(recipients) == 0 {
		return i18nconfig.FromConfig().Ef("smtp.no_recipient", nil)
	}
	client, err := dialSMTP(host, port, username, password, insecure)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Quit()
		_ = client.Close()
	}()

	message, err := buildMessage(from, recipients, subject, body)
	if err != nil {
		return err
	}

	envelopeFrom, err := envelopeAddr(from)
	if err != nil {
		return i18nconfig.FromConfig().Ewrap("smtp.envelope_from", nil, err)
	}
	if err := client.Mail(envelopeFrom); err != nil {
		return err
	}
	for _, rcpt := range recipients {
		envelopeTo, err := envelopeAddr(rcpt)
		if err != nil {
			return i18nconfig.FromConfig().Ewrap("smtp.envelope_to", map[string]string{"path": rcpt}, err)
		}
		if err := client.Rcpt(envelopeTo); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(message)); err != nil {
		return err
	}
	return w.Close()
}
