package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/the20100/meta-ad-library-cli/internal/api"
	"github.com/the20100/meta-ad-library-cli/internal/config"
	"github.com/the20100/meta-ad-library-cli/internal/metaauth"
)

var (
	jsonFlag   bool
	prettyFlag bool

	// Global API client, initialized in PersistentPreRunE.
	client *api.Client
	cfg    *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "meta-adlib",
	Short: "Meta Ad Library CLI — search and explore public Meta ads",
	Long: `meta-adlib is a CLI tool for the Meta Ad Library API.

It provides read-only access to the public Meta Ad Library, which contains
ads run on Facebook, Instagram, Messenger, and Audience Network.

Available for:
  • All ads targeting the European Union
  • Political and issue ads worldwide
  • Ads in Brazil (limited scope)

Token resolution order:
  1. META_TOKEN env var
  2. Own config    (~/.config/meta-ad-library/config.json  via: meta-adlib auth set-token)
  3. Shared config (~/.config/meta-auth/config.json        via: meta-auth login)

Examples:
  meta-auth login                                          (recommended: shared auth)
  meta-adlib search --query "climate change" --country US
  meta-adlib search --query "election" --country US --type POLITICAL_AND_ISSUE_ADS
  meta-adlib search --page-id 123456789 --country DE
  meta-adlib ad get <ad_archive_id>`,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Force JSON output")
	rootCmd.PersistentFlags().BoolVar(&prettyFlag, "pretty", false, "Force pretty-printed JSON output (implies --json)")
	rootCmd.AddCommand(infoCmd)
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if isAuthCommand(cmd) {
			return nil
		}

		token, err := resolveToken()
		if err != nil {
			return err
		}

		client = api.NewClient(token)
		return nil
	}
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show tool info: config paths, token status, and environment",
	Run: func(cmd *cobra.Command, args []string) {
		printInfo()
	},
}

func printInfo() {
	configDir, _ := os.UserConfigDir()
	ownConfig := filepath.Join(configDir, "meta-ad-library", "config.json")
	sharedConfig := filepath.Join(configDir, "meta-auth", "config.json")

	fmt.Println("meta-adlib — Meta Ad Library CLI")
	fmt.Println()

	exe, _ := os.Executable()
	fmt.Printf("  binary:  %s\n", exe)
	fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	fmt.Println("  config paths by OS:")
	fmt.Println("    macOS:    ~/Library/Application Support/meta-ad-library/config.json")
	fmt.Println("    Linux:    ~/.config/meta-ad-library/config.json")
	fmt.Println("    Windows:  %AppData%\\meta-ad-library\\config.json")
	fmt.Printf("  own config:    %s\n", ownConfig)
	fmt.Printf("  shared config: %s\n", sharedConfig)
	fmt.Println()

	// Token source
	tokenSource := "(not set)"
	userName := ""
	if t := os.Getenv("META_TOKEN"); t != "" {
		tokenSource = "META_TOKEN env var"
	} else if tok, name := readTokenFromFile(ownConfig); tok != "" {
		tokenSource = "own config"
		userName = name
	} else if tok, name := readTokenFromFile(sharedConfig); tok != "" {
		tokenSource = "meta-auth shared config"
		userName = name
	}
	fmt.Printf("  token source: %s\n", tokenSource)
	if userName != "" {
		fmt.Printf("  user:         %s\n", userName)
	}
	printExpiryFromFile(ownConfig, sharedConfig)

	fmt.Println()
	fmt.Println("  env vars:")
	fmt.Printf("    META_TOKEN = %s\n", maskOrEmpty(os.Getenv("META_TOKEN")))
	fmt.Println()
	fmt.Println("  token resolution order:")
	fmt.Println("    1. META_TOKEN env var")
	fmt.Println("    2. own config   (meta-adlib auth set-token)")
	fmt.Println("    3. shared config (meta-auth login)  ← recommended")
}

func readTokenFromFile(path string) (token, userName string) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) || err != nil {
		return "", ""
	}
	var cfg struct {
		AccessToken string `json:"access_token"`
		UserName    string `json:"user_name"`
	}
	if json.Unmarshal(data, &cfg) == nil {
		return cfg.AccessToken, cfg.UserName
	}
	return "", ""
}

func printExpiryFromFile(ownPath, sharedPath string) {
	for _, path := range []string{ownPath, sharedPath} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg struct {
			AccessToken    string `json:"access_token"`
			TokenExpiresAt int64  `json:"token_expires_at"`
		}
		if json.Unmarshal(data, &cfg) != nil || cfg.AccessToken == "" {
			continue
		}
		if cfg.TokenExpiresAt == 0 {
			fmt.Println("  expires:      unknown")
		} else {
			exp := time.Unix(cfg.TokenExpiresAt, 0)
			days := int(time.Until(exp).Hours() / 24)
			if days < 0 {
				fmt.Printf("  expires:      EXPIRED on %s\n", exp.Format("2006-01-02"))
			} else {
				fmt.Printf("  expires:      %s (%d days left)\n", exp.Format("2006-01-02"), days)
			}
		}
		return
	}
}

func maskOrEmpty(v string) string {
	if v == "" {
		return "(not set)"
	}
	if len(v) <= 8 {
		return "***"
	}
	return v[:4] + "..." + v[len(v)-4:]
}

// resolveToken returns the best available token using the priority chain.
func resolveToken() (string, error) {
	// 1. META_TOKEN env var (universal override for all Meta CLIs)
	if t := os.Getenv("META_TOKEN"); t != "" {
		return t, nil
	}

	// 2. Own config
	var err error
	cfg, err = config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.AccessToken != "" {
		warnOwnExpiry()
		return cfg.AccessToken, nil
	}

	// 3. meta-auth shared config
	sharedToken, err := metaauth.Token()
	if err != nil {
		return "", fmt.Errorf("failed to read meta-auth config: %w", err)
	}
	if sharedToken != "" {
		warnSharedExpiry()
		return sharedToken, nil
	}

	return "", fmt.Errorf("not authenticated — run: meta-auth login  (shared)\nor: meta-adlib auth set-token <token>  (local only)")
}

func warnOwnExpiry() {
	if cfg == nil {
		return
	}
	days := cfg.DaysUntilExpiry()
	switch {
	case cfg.IsExpired():
		fmt.Fprintf(os.Stderr, "warning: token has expired — run: meta-adlib auth refresh\n")
	case days >= 0 && days <= 7:
		fmt.Fprintf(os.Stderr, "warning: token expires in %d day(s) — run: meta-adlib auth refresh\n", days)
	}
}

func warnSharedExpiry() {
	days := metaauth.DaysUntilExpiry()
	switch {
	case metaauth.IsExpired():
		fmt.Fprintf(os.Stderr, "warning: meta-auth token has expired — run: meta-auth refresh\n")
	case days >= 0 && days <= 7:
		fmt.Fprintf(os.Stderr, "warning: meta-auth token expires in %d day(s) — run: meta-auth refresh\n", days)
	}
}

func isAuthCommand(cmd *cobra.Command) bool {
	if cmd.Name() == "auth" {
		return true
	}
	p := cmd.Parent()
	for p != nil {
		if p.Name() == "auth" {
			return true
		}
		p = p.Parent()
	}
	return false
}
