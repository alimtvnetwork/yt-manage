package mailer

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/alimtvnetwork/yt-manage/internal/config"
)

// FetchInbox returns the envelopes of the last limit messages in
// INBOX, newest first, using the IMAP settings in s.
func FetchInbox(s config.Settings, limit int) ([]Message, error) {
	if s.Mail.IMAPHost == "" {
		return nil, fmt.Errorf("IMAP is not configured (set mail settings in config.json)")
	}
	if limit <= 0 {
		limit = 20
	}
	port := s.Mail.IMAPPort
	if port == 0 {
		port = 993
	}
	addr := fmt.Sprintf("%s:%d", s.Mail.IMAPHost, port)
	options := &imapclient.Options{
		TLSConfig: &tls.Config{ServerName: s.Mail.IMAPHost},
	}
	client, err := imapclient.DialTLS(addr, options)
	if err != nil {
		return nil, fmt.Errorf("dial IMAP: %w", err)
	}
	defer client.Close()
	if err := client.Login(s.Mail.Username, s.Mail.Password).Wait(); err != nil {
		return nil, fmt.Errorf("IMAP login: %w", err)
	}
	selected, err := client.Select("INBOX", &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return nil, fmt.Errorf("select INBOX: %w", err)
	}
	if selected.NumMessages == 0 {
		return []Message{}, nil
	}
	start := uint32(1)
	total := selected.NumMessages
	if uint32(limit) < total {
		start = total - uint32(limit) + 1
	}
	var seqSet imap.SeqSet
	seqSet.AddRange(start, total)
	fetched, err := client.Fetch(seqSet, &imap.FetchOptions{Envelope: true, UID: true}).Collect()
	if err != nil {
		return nil, fmt.Errorf("fetch INBOX envelopes: %w", err)
	}
	messages := make([]Message, 0, len(fetched))
	for _, item := range fetched {
		if item.Envelope == nil {
			continue
		}
		messages = append(messages, Message{
			UID:     uint32(item.UID),
			From:    formatAddressList(item.Envelope.From),
			Subject: item.Envelope.Subject,
			Date:    item.Envelope.Date,
		})
	}
	// Fetched messages arrive oldest first; present newest first.
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	if err := client.Logout().Wait(); err != nil {
		return nil, fmt.Errorf("IMAP logout: %w", err)
	}
	return messages, nil
}

// formatAddressList renders envelope addresses as "Name <addr>"
// when a display name is present, otherwise the bare address.
func formatAddressList(addresses []imap.Address) string {
	parts := make([]string, 0, len(addresses))
	for _, address := range addresses {
		addr := address.Addr()
		if address.Name != "" && addr != "" {
			parts = append(parts, fmt.Sprintf("%s <%s>", address.Name, addr))
		} else if addr != "" {
			parts = append(parts, addr)
		} else if address.Name != "" {
			parts = append(parts, address.Name)
		}
	}
	return strings.Join(parts, ", ")
}
