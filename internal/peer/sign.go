// Package peer implements CLI-to-CLI exchange of mirrored cache
// rows (videos and playlists, keyed by their YouTube ids) between
// two yt-manage installations, using HMAC-SHA256 signed requests.
//
// Convention: both machines register the pair under the SAME peer
// name and the SAME shared secret. The sender identifies itself
// with the X-YTM-Peer header set to that name and signs the request
// body with the shared secret; the receiver looks the name up in
// its own peers table and verifies the signature against the secret
// it stored for that name.
package peer

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"

	"gorm.io/gorm"

	"github.com/alimtvnetwork/yt-manage/internal/repo"
)

// Sign returns the hex-encoded HMAC-SHA256 of body under secret.
func Sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify reports whether signature is the valid Sign of body under
// secret, using a constant-time comparison.
func Verify(secret string, body []byte, signature string) bool {
	expected := Sign(secret, body)
	if len(signature) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) == 1
}

// FindPeerSecret returns the shared secret stored for the peer with
// the given name. The second return value is false (with a nil
// error) when no peer with that name is registered.
func FindPeerSecret(store *repo.Store, name string) (string, bool, error) {
	peerRow, err := store.Peers.GetByName(name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return peerRow.SharedSecret, true, nil
}
