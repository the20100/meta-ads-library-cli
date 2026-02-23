package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/vincentmaurin/meta-ad-library-cli/internal/config"
)

const (
	metaMeURL       = "https://graph.facebook.com/v23.0/me"
	metaExchangeURL = "https://graph.facebook.com/v23.0/oauth/access_token"
)

var authSetTokenNoExtend bool
var authExtendTokenSave bool

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Meta Ad Library authentication",
}

var authSetTokenCmd = &cobra.Command{
	Use:   "set-token <token>",
	Short: "Save a Meta access token",
	Long: `Saves a Meta user access token to the config file.

The token is validated by calling GET /me. If META_APP_ID and META_APP_SECRET
are set (env vars), the token is automatically upgraded to a long-lived token
(~60 days) unless --no-extend is passed.

You can obtain a short-lived token from:
  • Meta Graph API Explorer: https://developers.facebook.com/tools/explorer/

Examples:
  meta-adlib auth set-token EAABsbCS...
  meta-adlib auth set-token EAABsbCS... --no-extend
  META_APP_ID=123 META_APP_SECRET=abc meta-adlib auth set-token EAABsbCS...`,
	Args: cobra.ExactArgs(1),
	RunE: runAuthSetToken,
}

var authExtendTokenCmd = &cobra.Command{
	Use:   "extend-token <short_lived_token>",
	Short: "Exchange a short-lived token for a long-lived one (~60 days)",
	Long: `Calls the Meta token exchange endpoint to upgrade a short-lived user
access token to a long-lived one that expires in approximately 60 days.

Requires META_APP_ID and META_APP_SECRET environment variables.

Examples:
  # Print the long-lived token only
  meta-adlib auth extend-token EAABsbCS...

  # Extend AND save to config
  meta-adlib auth extend-token EAABsbCS... --save`,
	Args: cobra.ExactArgs(1),
	RunE: runAuthExtendToken,
}

var authRefreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh the stored token before it expires",
	Long: `Re-exchanges the currently stored token for a fresh long-lived token (~60 days).

This resets the 60-day expiry window from today, so you never need to log in
again as long as you refresh before the token expires.

Requires META_APP_ID and META_APP_SECRET environment variables.

Run this periodically (e.g. once a month via cron) to keep the token alive:
  0 9 1 * * META_APP_ID=... META_APP_SECRET=... meta-adlib auth refresh

Examples:
  meta-adlib auth refresh
  META_APP_ID=123 META_APP_SECRET=abc meta-adlib auth refresh`,
	RunE: runAuthRefresh,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove saved credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Clear(); err != nil {
			return fmt.Errorf("failed to clear config: %w", err)
		}
		fmt.Println("logged out")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		if c.AccessToken == "" {
			fmt.Println("not authenticated")
			fmt.Println("  → meta-adlib auth set-token <token>")
			fmt.Println("  → export META_ADLIB_TOKEN=<token>")
			return nil
		}

		fmt.Printf("authenticated as %s (ID: %s)\n", c.UserName, c.UserID)

		days := c.DaysUntilExpiry()
		switch {
		case days == -1:
			fmt.Println("  expires:  unknown (token may never expire, or expiry not tracked)")
		case c.IsExpired():
			fmt.Printf("  expires:  EXPIRED on %s — run: meta-adlib auth refresh\n",
				c.ExpiresAt().Format("2006-01-02"))
		case days <= 7:
			fmt.Printf("  expires:  %s (%d day(s) left) ⚠️  — run: meta-adlib auth refresh\n",
				c.ExpiresAt().Format("2006-01-02"), days)
		default:
			fmt.Printf("  expires:  %s (%d days left)\n",
				c.ExpiresAt().Format("2006-01-02"), days)
		}

		fmt.Printf("  config:   %s\n", config.Path())
		return nil
	},
}

func init() {
	authSetTokenCmd.Flags().BoolVar(&authSetTokenNoExtend, "no-extend", false, "Skip upgrading to long-lived token even if app credentials are available")
	authExtendTokenCmd.Flags().BoolVar(&authExtendTokenSave, "save", false, "Save the long-lived token to config (replaces current token)")

	authCmd.AddCommand(authSetTokenCmd, authExtendTokenCmd, authRefreshCmd, authLogoutCmd, authStatusCmd)
	rootCmd.AddCommand(authCmd)
}

// ── handlers ──────────────────────────────────────────────────────────────────

func runAuthSetToken(cmd *cobra.Command, args []string) error {
	token := args[0]

	appID := os.Getenv("META_APP_ID")
	appSecret := os.Getenv("META_APP_SECRET")

	finalToken := token
	var expiresAt int64

	// Auto-upgrade to long-lived if app credentials are available
	if !authSetTokenNoExtend && appID != "" && appSecret != "" {
		fmt.Println("app credentials found — upgrading to long-lived token (~60 days)...")
		lt, exp, err := exchangeToLongLived(token, appID, appSecret)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not upgrade to long-lived token: %v\n", err)
			fmt.Fprintf(os.Stderr, "         saving original token. Use --no-extend to suppress this warning.\n")
		} else {
			finalToken = lt
			expiresAt = exp
			fmt.Println("token upgraded to long-lived")
		}
	} else if !authSetTokenNoExtend && (appID == "" || appSecret == "") {
		fmt.Fprintln(os.Stderr, "note: META_APP_ID / META_APP_SECRET not set — saving token as-is (not extended)")
		fmt.Fprintln(os.Stderr, "      to extend later: meta-adlib auth extend-token <token> --save")
	}

	fmt.Println("validating token...")
	userID, userName, err := fetchMe(finalToken)
	if err != nil {
		return fmt.Errorf("token validation failed: %w", err)
	}

	newCfg := &config.Config{
		AccessToken:    finalToken,
		UserID:         userID,
		UserName:       userName,
		TokenExpiresAt: expiresAt,
	}

	if err := config.Save(newCfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("token saved — authenticated as %s (ID: %s)\n", userName, userID)
	if expiresAt != 0 {
		fmt.Printf("  expires: %s (%d days)\n",
			time.Unix(expiresAt, 0).Format("2006-01-02"),
			newCfg.DaysUntilExpiry())
	}
	fmt.Printf("  config:  %s\n", config.Path())
	return nil
}

func runAuthExtendToken(cmd *cobra.Command, args []string) error {
	shortToken := args[0]

	appID := os.Getenv("META_APP_ID")
	appSecret := os.Getenv("META_APP_SECRET")

	if appID == "" {
		return fmt.Errorf("META_APP_ID not set — export META_APP_ID=<your_app_id>")
	}
	if appSecret == "" {
		return fmt.Errorf("META_APP_SECRET not set — export META_APP_SECRET=<your_app_secret>")
	}

	fmt.Println("exchanging for long-lived token...")
	longToken, expiresAt, err := exchangeToLongLived(shortToken, appID, appSecret)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	if authExtendTokenSave {
		fmt.Println("validating token...")
		userID, userName, err := fetchMe(longToken)
		if err != nil {
			return fmt.Errorf("token validation failed: %w", err)
		}

		newCfg := &config.Config{
			AccessToken:    longToken,
			UserID:         userID,
			UserName:       userName,
			TokenExpiresAt: expiresAt,
		}
		if err := config.Save(newCfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		fmt.Printf("long-lived token saved — authenticated as %s (ID: %s)\n", userName, userID)
		if expiresAt != 0 {
			fmt.Printf("  expires: %s (%d days)\n",
				time.Unix(expiresAt, 0).Format("2006-01-02"),
				newCfg.DaysUntilExpiry())
		}
		fmt.Printf("  config:  %s\n", config.Path())
	} else {
		fmt.Printf("\nlong-lived token:\n%s\n", longToken)
		if expiresAt != 0 {
			fmt.Printf("expires: %s\n", time.Unix(expiresAt, 0).Format("2006-01-02"))
		}
		fmt.Println("\nto save it to config, run:")
		fmt.Printf("  meta-adlib auth set-token %s\n", longToken)
		fmt.Println("or re-run with --save:")
		fmt.Println("  meta-adlib auth extend-token <short_token> --save")
	}
	return nil
}

func runAuthRefresh(cmd *cobra.Command, args []string) error {
	appID := os.Getenv("META_APP_ID")
	appSecret := os.Getenv("META_APP_SECRET")

	if appID == "" {
		return fmt.Errorf("META_APP_ID not set — export META_APP_ID=<your_app_id>")
	}
	if appSecret == "" {
		return fmt.Errorf("META_APP_SECRET not set — export META_APP_SECRET=<your_app_secret>")
	}

	c, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if c.AccessToken == "" {
		return fmt.Errorf("not authenticated — run: meta-adlib auth set-token <token>")
	}

	// Show current expiry before refreshing
	days := c.DaysUntilExpiry()
	if days == -1 {
		fmt.Println("refreshing token (current expiry unknown)...")
	} else if c.IsExpired() {
		fmt.Println("token has expired — attempting refresh anyway...")
	} else {
		fmt.Printf("current token expires in %d day(s) — refreshing now...\n", days)
	}

	newToken, expiresAt, err := exchangeToLongLived(c.AccessToken, appID, appSecret)
	if err != nil {
		return fmt.Errorf("token refresh failed: %w", err)
	}

	newCfg := &config.Config{
		AccessToken:    newToken,
		UserID:         c.UserID,
		UserName:       c.UserName,
		TokenExpiresAt: expiresAt,
	}
	if err := config.Save(newCfg); err != nil {
		return fmt.Errorf("failed to save refreshed token: %w", err)
	}

	fmt.Printf("token refreshed — authenticated as %s\n", c.UserName)
	if expiresAt != 0 {
		fmt.Printf("  new expiry: %s (%d days)\n",
			time.Unix(expiresAt, 0).Format("2006-01-02"),
			newCfg.DaysUntilExpiry())
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// tokenResponse is the shape of Meta's token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"` // seconds until expiry
	Error       *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// exchangeToLongLived upgrades a token to a ~60-day long-lived token.
// Returns (token, expiresAtUnix, error). expiresAtUnix is 0 if not provided by Meta.
func exchangeToLongLived(shortToken, appID, appSecret string) (string, int64, error) {
	params := url.Values{}
	params.Set("grant_type", "fb_exchange_token")
	params.Set("client_id", appID)
	params.Set("client_secret", appSecret)
	params.Set("fb_exchange_token", shortToken)

	return metaTokenFetch(metaExchangeURL + "?" + params.Encode())
}

// metaTokenFetch performs a GET to a Meta token endpoint and returns
// (accessToken, expiresAtUnix, error).
func metaTokenFetch(reqURL string) (string, int64, error) {
	resp, err := http.Get(reqURL) //nolint:noctx
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}

	var result tokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, fmt.Errorf("parsing token response: %w", err)
	}
	if result.Error != nil {
		return "", 0, fmt.Errorf("meta api error: %s", result.Error.Message)
	}
	if result.AccessToken == "" {
		return "", 0, fmt.Errorf("no access_token in response: %s", string(body))
	}

	var expiresAt int64
	if result.ExpiresIn > 0 {
		expiresAt = time.Now().Unix() + result.ExpiresIn
	}

	return result.AccessToken, expiresAt, nil
}

// fetchMe calls GET /me and returns (userID, userName, error).
func fetchMe(token string) (string, string, error) {
	params := url.Values{}
	params.Set("access_token", token)
	params.Set("fields", "id,name")

	resp, err := http.Get(metaMeURL + "?" + params.Encode()) //nolint:noctx
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var result struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("parsing /me response: %w", err)
	}
	if result.Error != nil {
		return "", "", fmt.Errorf("meta api error: %s", result.Error.Message)
	}
	return result.ID, result.Name, nil
}
