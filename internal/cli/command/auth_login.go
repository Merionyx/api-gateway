package command

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	apiserverclient "github.com/merionyx/api-gateway/internal/cli/apiserver/client"
	"github.com/merionyx/api-gateway/internal/cli/config"
	"github.com/merionyx/api-gateway/internal/cli/credentials"

	"github.com/spf13/cobra"
)

const (
	defaultCallbackHost = "127.0.0.1"
	defaultCallbackPort = 21987
	callbackPath        = "/callback"
)

func newAuthLoginCmd(resolveServer func() (string, error)) *cobra.Command {
	var (
		providerID   string
		callbackHost string
		callbackPort int
		noBrowser    bool
		accessTTL    string
		refreshTTL   string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Browser OIDC login: loopback callback, save tokens to credentials file (0600)",
		Long: strings.TrimSpace(`
Opens the system browser for API Server GET /v1/auth/authorize, then receives the OAuth redirect on a local HTTP listener.

Without --provider-id, agwctl calls GET /v1/auth/oidc-providers: if there is a single provider it is chosen automatically; if there are several, you pick one from an interactive list (arrow keys + Enter) when stdin/stdout are a TTY—otherwise pass --provider-id explicitly.

You must add the exact redirect URI to auth.oidc_providers[].redirect_uri_allowlist for this provider (default:
  http://127.0.0.1:21987/callback
). Override host/port with --callback-host / --callback-port if that port is busy.

Tokens are written to ~/.config/agwctl/credentials.yaml (or AGWCTL_CREDENTIALS), keyed by the current agwctl context; file mode 0600.
`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctxName, err := effectiveContextName(cmd)
			if err != nil {
				return err
			}
			server, err := resolveServer()
			if err != nil {
				return err
			}
			baseHTTP, err := httpClientFromCmd(cmd)
			if err != nil {
				return err
			}
			execCtx := cmd.Context()
			if execCtx == nil {
				execCtx = context.Background()
			}
			return runAuthLogin(
				execCtx,
				cmd.OutOrStdout(),
				server,
				baseHTTP,
				ctxName,
				providerID,
				callbackHost,
				callbackPort,
				noBrowser,
				accessTTL,
				refreshTTL,
			)
		},
	}
	cmd.Flags().StringVar(&providerID, "provider-id", "", "OIDC provider id (auth.oidc_providers[].id); optional if the server exposes a single provider or you pick interactively")
	cmd.Flags().StringVar(&callbackHost, "callback-host", defaultCallbackHost, "loopback host for redirect_uri (must match allowlist)")
	cmd.Flags().IntVar(&callbackPort, "callback-port", defaultCallbackPort, "TCP port for loopback redirect_uri (must match allowlist)")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "print IdP URL instead of opening a browser")
	cmd.Flags().StringVar(&accessTTL, "access-ttl", "", "requested access token lifetime (default 168h; Go duration or seconds, e.g. 168h or 604800)")
	cmd.Flags().StringVar(&refreshTTL, "refresh-ttl", "", "requested refresh token lifetime (default 720h; Go duration or seconds, e.g. 720h or 2592000)")
	return cmd
}

func runAuthLogin(
	ctx context.Context,
	out io.Writer,
	server string,
	baseHTTP *http.Client,
	contextName string,
	providerID string,
	callbackHost string,
	callbackPort int,
	noBrowser bool,
	accessTTL string,
	refreshTTL string,
) error {
	chosenID, err := resolveAuthLoginProviderID(ctx, server, baseHTTP, providerID, out)
	if err != nil {
		return err
	}
	requestedTTLs, err := requestedTTLsFromFlags(accessTTL, refreshTTL)
	if err != nil {
		return err
	}
	requestedTTLs = withDefaultRequestedTTLs(requestedTTLs)
	codeVerifier, codeChallenge, err := newOAuthPKCES256()
	if err != nil {
		return err
	}
	clientState, err := newOAuthState()
	if err != nil {
		return err
	}

	redirectURI := fmt.Sprintf("http://%s:%d%s", strings.TrimSpace(callbackHost), callbackPort, callbackPath)
	ln, err := net.Listen("tcp", net.JoinHostPort(callbackHost, fmt.Sprintf("%d", callbackPort)))
	if err != nil {
		return fmt.Errorf("listen %s: %w (choose another --callback-port and add that URI to the server's redirect_uri_allowlist)", redirectURI, err)
	}
	defer func() { _ = ln.Close() }()

	type callbackData struct {
		code  string
		state string
		err   string
	}
	ch := make(chan callbackData, 1)

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		code := strings.TrimSpace(q.Get("code"))
		state := strings.TrimSpace(q.Get("state"))
		if errParam := strings.TrimSpace(q.Get("error")); errParam != "" {
			desc := strings.TrimSpace(q.Get("error_description"))
			select {
			case ch <- callbackData{err: fmt.Sprintf("%s: %s", errParam, desc)}:
			default:
			}
			http.Error(w, "login failed", http.StatusBadRequest)
			return
		}
		if code == "" || state == "" {
			select {
			case ch <- callbackData{err: "missing code or state"}:
			default:
			}
			http.Error(w, "missing code or state", http.StatusBadRequest)
			return
		}
		select {
		case ch <- callbackData{code: code, state: state}:
		default:
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><html><body><p>Login complete. You can close this tab and return to the terminal.</p></body></html>\n")
	})

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
	}()

	tr := http.DefaultTransport
	if baseHTTP.Transport != nil {
		tr = baseHTTP.Transport
	}
	noRedirect := &http.Client{
		Transport:     tr,
		Timeout:       baseHTTP.Timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
		Jar:           baseHTTP.Jar,
	}
	apiNoRedir, err := apiserverclient.NewClient(server, apiserverclient.WithHTTPClient(noRedirect))
	if err != nil {
		return err
	}
	pid := chosenID
	redirectURIForToken := redirectURI
	clientID := "agwctl"
	state := clientState
	authorizeResp, err := apiNoRedir.AuthorizeOidc(ctx, &apiserverclient.AuthorizeOidcParams{
		ProviderId:          &pid,
		RedirectUri:         redirectURI,
		ResponseType:        apiserverclient.Code,
		ClientId:            clientID,
		State:               &state,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: apiserverclient.S256,
	})
	if err != nil {
		return fmt.Errorf("authorize request: %w", err)
	}
	defer func() { _ = authorizeResp.Body.Close() }()
	if authorizeResp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(io.LimitReader(authorizeResp.Body, 4096))
		return fmt.Errorf("authorize: expected HTTP 302, got %d: %s", authorizeResp.StatusCode, string(body))
	}
	idpURL := strings.TrimSpace(authorizeResp.Header.Get("Location"))
	if idpURL == "" {
		return fmt.Errorf("authorize: empty Location header")
	}

	if noBrowser {
		_, _ = fmt.Fprintf(out, "Open this URL in your browser (no-browser mode):\n%s\n", idpURL)
	} else {
		if err := openBrowser(idpURL); err != nil {
			return fmt.Errorf("open browser: %w", err)
		}
		_, _ = fmt.Fprintf(out, "Opened browser. Waiting for callback on %s ...\n", redirectURI)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 12*time.Minute)
	defer cancel()
	var cb callbackData
	select {
	case <-waitCtx.Done():
		return fmt.Errorf("timeout waiting for browser callback on %s", redirectURI)
	case cb = <-ch:
	}
	if cb.err != "" {
		return fmt.Errorf("callback: %s", cb.err)
	}
	if subtleTrimEqualLogin(cb.state, clientState) == 0 {
		return fmt.Errorf("callback: state mismatch")
	}

	apiClient, err := apiserverclient.NewClientWithResponses(server, apiserverclient.WithHTTPClient(baseHTTP))
	if err != nil {
		return err
	}
	tokenResp, err := apiClient.TokenOidcWithFormdataBodyWithResponse(waitCtx, apiserverclient.TokenOidcFormdataRequestBody{
		GrantType:    apiserverclient.AuthorizationCode,
		Code:         &cb.code,
		RedirectUri:  &redirectURIForToken,
		ClientId:     &clientID,
		CodeVerifier: &codeVerifier,
		AccessTtl:    optionalSeconds(requestedTTLs.AccessTTL),
		RefreshTtl:   optionalSeconds(requestedTTLs.RefreshTTL),
	})
	if err != nil {
		return fmt.Errorf("token endpoint: %w", err)
	}
	if tokenResp.JSON200 == nil {
		if tokenResp.JSON400 != nil {
			return fmt.Errorf("token endpoint: %s", oauthTokenErrorString(tokenResp.JSON400))
		}
		if tokenResp.JSON503 != nil {
			return fmt.Errorf("token endpoint unavailable: %s", oauthTokenErrorString(tokenResp.JSON503))
		}
		if tokenResp.JSON500 != nil {
			return fmt.Errorf("token endpoint error: %s", oauthTokenErrorString(tokenResp.JSON500))
		}
		body := string(tokenResp.Body)
		if len(body) > 2048 {
			body = body[:2048] + "…"
		}
		return fmt.Errorf("token endpoint: HTTP %d: %s", tokenResp.StatusCode(), body)
	}

	tok := tokenResp.JSON200
	tt := strings.TrimSpace(tok.Data.TokenType)
	if tt == "" {
		tt = "Bearer"
	}
	refreshToken := ""
	if tok.Data.RefreshToken != nil {
		refreshToken = strings.TrimSpace(*tok.Data.RefreshToken)
	}
	if refreshToken == "" {
		return fmt.Errorf("token endpoint: refresh_token is missing")
	}
	accessExpiresAt := time.Now().UTC().Add(time.Duration(tok.Data.ExpiresIn) * time.Second)
	if tok.Data.AccessExpiresAt != nil {
		accessExpiresAt = tok.Data.AccessExpiresAt.UTC()
	}
	refreshExpiresAt := accessExpiresAt
	if tok.Data.RefreshExpiresAt != nil {
		refreshExpiresAt = tok.Data.RefreshExpiresAt.UTC()
	} else if tok.Data.RefreshExpiresIn != nil && *tok.Data.RefreshExpiresIn > 0 {
		refreshExpiresAt = time.Now().UTC().Add(time.Duration(*tok.Data.RefreshExpiresIn) * time.Second)
	}
	if err := credentials.PutContext(contextName, credentials.Entry{
		ProviderID:               chosenID,
		AccessToken:              tok.Data.AccessToken,
		RefreshToken:             refreshToken,
		TokenType:                tt,
		AccessExpiresAt:          tok.Data.AccessExpiresAt.UTC().Format(time.RFC3339),
		RefreshExpiresAt:         refreshExpiresAt.UTC().Format(time.RFC3339),
		RequestedAccessTokenTTL:  resolvedTTLString(accessTTL, requestedTTLs.AccessTTL),
		RequestedRefreshTokenTTL: resolvedTTLString(refreshTTL, requestedTTLs.RefreshTTL),
	}); err != nil {
		return err
	}
	credPath, perr := credentials.Path()
	if perr != nil {
		credPath = "(credentials path)"
	}
	_, _ = fmt.Fprintf(out, "Saved tokens for context %q to %s (mode 0600).\n", contextName, credPath)
	return nil
}

func newOAuthPKCES256() (verifier, challenge string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("pkce verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func newOAuthState() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("oauth state: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

func oauthTokenErrorString(e *apiserverclient.OAuthTokenError) string {
	if e == nil {
		return ""
	}
	msg := strings.TrimSpace(e.Error)
	if e.ErrorDescription != nil && strings.TrimSpace(*e.ErrorDescription) != "" {
		if msg != "" {
			return msg + ": " + strings.TrimSpace(*e.ErrorDescription)
		}
		return strings.TrimSpace(*e.ErrorDescription)
	}
	return msg
}

func subtleTrimEqualLogin(a, b string) int {
	aa := strings.TrimSpace(a)
	bb := strings.TrimSpace(b)
	if len(aa) != len(bb) {
		return 0
	}
	var diff byte
	for i := 0; i < len(aa); i++ {
		diff |= aa[i] ^ bb[i]
	}
	if diff == 0 {
		return 1
	}
	return 0
}

func effectiveContextName(cmd *cobra.Command) (string, error) {
	ctx, err := cmd.Root().PersistentFlags().GetString("context")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(ctx) != "" {
		return strings.TrimSpace(ctx), nil
	}
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(cfg.CurrentContext)
	if name == "" {
		return "", fmt.Errorf("no context: run `agwctl config use-context NAME` or pass --context")
	}
	return name, nil
}

func openBrowser(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("refusing to open non-http(s) URL")
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", raw).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", raw).Start()
	default:
		return exec.Command("xdg-open", raw).Start()
	}
}
