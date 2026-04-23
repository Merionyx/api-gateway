package command

import (
	"context"
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
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Browser OIDC login: loopback callback, save tokens to credentials file (0600)",
		Long: strings.TrimSpace(`
Opens the system browser for API Server GET /api/v1/auth/login, then receives the IdP redirect on a local HTTP listener.

You must add the exact redirect URI to auth.oidc_providers[].redirect_uri_allowlist for this provider (default:
  http://127.0.0.1:21987/callback
). Override host/port with --callback-host / --callback-port if that port is busy.

Tokens are written to ~/.config/agwctl/credentials.yaml (or AGWCTL_CREDENTIALS), keyed by the current agwctl context; file mode 0600.
`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(providerID) == "" {
				return fmt.Errorf("--provider-id is required")
			}
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
			loginResp, err := apiNoRedir.LoginOidc(execCtx, &apiserverclient.LoginOidcParams{
				ProviderId:  providerID,
				RedirectUri: redirectURI,
			})
			if err != nil {
				return fmt.Errorf("login request: %w", err)
			}
			defer func() { _ = loginResp.Body.Close() }()
			if loginResp.StatusCode != http.StatusFound {
				body, _ := io.ReadAll(io.LimitReader(loginResp.Body, 4096))
				return fmt.Errorf("login: expected HTTP 302, got %d: %s", loginResp.StatusCode, string(body))
			}
			idpURL := strings.TrimSpace(loginResp.Header.Get("Location"))
			if idpURL == "" {
				return fmt.Errorf("login: empty Location header")
			}

			out := cmd.OutOrStdout()
			if noBrowser {
				_, _ = fmt.Fprintf(out, "Open this URL in your browser (no-browser mode):\n%s\n", idpURL)
			} else {
				if err := openBrowser(idpURL); err != nil {
					return fmt.Errorf("open browser: %w", err)
				}
				_, _ = fmt.Fprintf(out, "Opened browser. Waiting for callback on %s ...\n", redirectURI)
			}

			waitCtx, cancel := context.WithTimeout(execCtx, 12*time.Minute)
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

			apiClient, err := apiserverclient.NewClient(server, apiserverclient.WithHTTPClient(baseHTTP))
			if err != nil {
				return err
			}
			cbResp, err := apiClient.CallbackOidc(waitCtx, &apiserverclient.CallbackOidcParams{
				Code:  cb.code,
				State: cb.state,
			})
			if err != nil {
				return fmt.Errorf("callback API: %w", err)
			}
			parsed, err := apiserverclient.ParseCallbackOidcResponse(cbResp)
			if err != nil {
				return err
			}
			if parsed.JSON200 == nil {
				body := string(parsed.Body)
				if len(body) > 2048 {
					body = body[:2048] + "…"
				}
				return fmt.Errorf("callback: HTTP %d: %s", cbResp.StatusCode, body)
			}
			tok := parsed.JSON200
			tt := "Bearer"
			if tok.TokenType != nil && strings.TrimSpace(*tok.TokenType) != "" {
				tt = strings.TrimSpace(*tok.TokenType)
			}
			if err := credentials.PutContext(ctxName, credentials.Entry{
				ProviderID:   providerID,
				AccessToken:  tok.AccessToken,
				RefreshToken: tok.RefreshToken,
				TokenType:    tt,
			}); err != nil {
				return err
			}
			credPath, perr := credentials.Path()
			if perr != nil {
				credPath = "(credentials path)"
			}
			_, _ = fmt.Fprintf(out, "Saved tokens for context %q to %s (mode 0600).\n", ctxName, credPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&providerID, "provider-id", "", "OIDC provider id (auth.oidc_providers[].id on the API Server)")
	cmd.Flags().StringVar(&callbackHost, "callback-host", defaultCallbackHost, "loopback host for redirect_uri (must match allowlist)")
	cmd.Flags().IntVar(&callbackPort, "callback-port", defaultCallbackPort, "TCP port for loopback redirect_uri (must match allowlist)")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "print IdP URL instead of opening a browser")
	_ = cmd.MarkFlagRequired("provider-id")
	return cmd
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
