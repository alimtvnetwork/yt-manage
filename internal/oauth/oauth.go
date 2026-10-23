// Package oauth loads the Google OAuth client configuration and runs
// the loopback consent flow that yields a refresh token for a
// connected YouTube account.
package oauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/alimtvnetwork/yt-manage/internal/config"
)

// ScopeForceSSL is the only scope yt-manage requests: it covers
// reading and updating YouTube data and inserting comments.
const ScopeForceSSL = "https://www.googleapis.com/auth/youtube.force-ssl"

// LoadClientConfig builds the OAuth client configuration. When both
// YT_MANAGE_CLIENT_ID and YT_MANAGE_CLIENT_SECRET are set they win;
// when exactly one is set an error names the missing one. Otherwise
// the client-secret JSON at config.ClientSecretPath(home) is read
// (both the "installed" and "web" shapes are accepted) and the scope
// is forced to ScopeForceSSL.
func LoadClientConfig(home string) (*oauth2.Config, error) {
	clientID := os.Getenv("YT_MANAGE_CLIENT_ID")
	clientSecret := os.Getenv("YT_MANAGE_CLIENT_SECRET")
	hasClientID := clientID != ""
	hasClientSecret := clientSecret != ""
	if hasClientID && hasClientSecret {
		return &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     google.Endpoint,
			Scopes:       []string{ScopeForceSSL},
		}, nil
	}
	if hasClientID && !hasClientSecret {
		return nil, fmt.Errorf("YT_MANAGE_CLIENT_ID is set but YT_MANAGE_CLIENT_SECRET is missing: set both environment variables or neither")
	}
	if !hasClientID && hasClientSecret {
		return nil, fmt.Errorf("YT_MANAGE_CLIENT_SECRET is set but YT_MANAGE_CLIENT_ID is missing: set both environment variables or neither")
	}
	path := config.ClientSecretPath(home)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no OAuth client configured: set YT_MANAGE_CLIENT_ID and YT_MANAGE_CLIENT_SECRET, or place a client_secret.json in the data dir at %s", path)
		}
		return nil, fmt.Errorf("read client secret file: %w", err)
	}
	cfg, err := google.ConfigFromJSON(data, ScopeForceSSL)
	if err != nil {
		return nil, fmt.Errorf("parse client secret file %s: %w", path, err)
	}
	cfg.Scopes = []string{ScopeForceSSL}
	return cfg, nil
}

// callbackResult carries the outcome of the single loopback callback
// request back to Authorize.
type callbackResult struct {
	code string
	err  error
}

// Authorize runs the consent flow: it listens on a random loopback
// port, prints the authorisation URL to out, waits for the single
// callback request (verifying the random state), exchanges the code,
// and returns the token. The token must carry a refresh token, which
// only happens when consent grants offline access.
func Authorize(ctx context.Context, cfg *oauth2.Config, out io.Writer) (*oauth2.Token, error) {
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("generate oauth state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for oauth callback: %w", err)
	}
	addr, hasTCPAddr := listener.Addr().(*net.TCPAddr)
	if !hasTCPAddr {
		_ = listener.Close()
		return nil, fmt.Errorf("listen for oauth callback: unexpected address %v", listener.Addr())
	}
	cfg.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/callback", addr.Port)

	resultCh := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("error") != "" {
			sendCallbackResult(resultCh, callbackResult{err: fmt.Errorf("oauth consent failed: %s", query.Get("error"))})
			http.Error(w, "Authorisation failed. You can close this window.", http.StatusBadRequest)
			return
		}
		if query.Get("state") != state {
			sendCallbackResult(resultCh, callbackResult{err: fmt.Errorf("oauth callback state mismatch")})
			http.Error(w, "State mismatch. You can close this window.", http.StatusBadRequest)
			return
		}
		code := query.Get("code")
		if code == "" {
			sendCallbackResult(resultCh, callbackResult{err: fmt.Errorf("oauth callback returned no code")})
			http.Error(w, "No code returned. You can close this window.", http.StatusBadRequest)
			return
		}
		sendCallbackResult(resultCh, callbackResult{code: code})
		fmt.Fprintln(w, "Authorised. You can close this window and return to yt-manage.")
	})

	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()

	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Fprintf(out, "Open this URL in your browser to connect your YouTube account:\n\n%s\n\nWaiting for authorisation (up to 5 minutes)...\n", authURL)

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var result callbackResult
	select {
	case result = <-resultCh:
	case <-waitCtx.Done():
		_ = server.Close()
		return nil, fmt.Errorf("wait for oauth callback: %w", waitCtx.Err())
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)

	if result.err != nil {
		return nil, result.err
	}
	token, err := cfg.Exchange(ctx, result.code)
	if err != nil {
		return nil, fmt.Errorf("exchange oauth code: %w", err)
	}
	if token.RefreshToken == "" {
		return nil, fmt.Errorf("oauth token has no refresh token: consent must grant offline access, so re-run and approve offline access when prompted")
	}
	return token, nil
}

// sendCallbackResult delivers the first callback outcome without
// blocking the HTTP handler on any later stray request.
func sendCallbackResult(ch chan callbackResult, result callbackResult) {
	select {
	case ch <- result:
	default:
	}
}
