package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vincentmaurin/meta-ad-library-cli/internal/api"
	"github.com/vincentmaurin/meta-ad-library-cli/internal/output"
)

// All available fields for /ads_archive (funding_entity deprecated since v13)
const defaultFields = "id,ad_creation_time,ad_delivery_start_time,ad_delivery_stop_time," +
	"ad_creative_bodies,ad_creative_link_titles,ad_creative_link_captions," +
	"ad_snapshot_url,page_id,page_name,publisher_platforms,languages," +
	"spend,impressions,currency"

var (
	searchQuery      string
	searchCountries  []string
	searchPageIDs    []string
	searchAdType     string
	searchStatus     string
	searchDateMin    string
	searchDateMax    string
	searchPlatforms  []string
	searchLanguages  []string
	searchLimit      int
	searchFields     string
	searchMediaType  string
)

var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search the Meta Ad Library",
	Long: `Search the Meta Ad Library via the /ads_archive endpoint.

At least one of --query or --page-id is required.
At least one --country is required.

Ad types:
  ALL                      All ads (default)
  POLITICAL_AND_ISSUE_ADS  Political/issue ads (required for some regions)

Status values:
  ALL     Active and inactive ads (default)
  ACTIVE  Currently running ads only

Platforms:
  facebook, instagram, audience_network, messenger, threads

Examples:
  meta-adlib search --query "climate" --country US
  meta-adlib search --query "election" --country US --type POLITICAL_AND_ISSUE_ADS --status ACTIVE
  meta-adlib search --page-id 123456789 --country DE --limit 50
  meta-adlib search --query "cars" --country FR --country DE --platform facebook --platform instagram
  meta-adlib search --query "health" --country US --since 2024-01-01 --until 2024-12-31
  meta-adlib search --query "shoes" --country US --json`,
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().StringVar(&searchQuery, "query", "", "Search terms to find in ad creative text")
	searchCmd.Flags().StringArrayVar(&searchCountries, "country", nil, "Country code(s) (ISO 3166, e.g. US, DE, FR). Repeatable.")
	searchCmd.Flags().StringArrayVar(&searchPageIDs, "page-id", nil, "Facebook Page ID(s) to search. Repeatable.")
	searchCmd.Flags().StringVar(&searchAdType, "type", "ALL", "Ad type: ALL or POLITICAL_AND_ISSUE_ADS")
	searchCmd.Flags().StringVar(&searchStatus, "status", "ALL", "Ad active status: ALL or ACTIVE")
	searchCmd.Flags().StringVar(&searchDateMin, "since", "", "Minimum delivery start date (YYYY-MM-DD)")
	searchCmd.Flags().StringVar(&searchDateMax, "until", "", "Maximum delivery start date (YYYY-MM-DD)")
	searchCmd.Flags().StringArrayVar(&searchPlatforms, "platform", nil, "Platform filter: facebook, instagram, audience_network, messenger, threads. Repeatable.")
	searchCmd.Flags().StringArrayVar(&searchLanguages, "language", nil, "Language filter (ISO 639-1, e.g. en, fr). Repeatable.")
	searchCmd.Flags().IntVar(&searchLimit, "limit", 25, "Maximum number of results (0 = fetch all pages)")
	searchCmd.Flags().StringVar(&searchFields, "fields", defaultFields, "Comma-separated list of fields to return")
	searchCmd.Flags().StringVar(&searchMediaType, "media-type", "", "Filter by media type: ALL, IMAGE, MEME, VIDEO, NONE")

	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	if len(searchCountries) == 0 {
		return fmt.Errorf("at least one --country is required (e.g. --country US)")
	}
	if searchQuery == "" && len(searchPageIDs) == 0 {
		return fmt.Errorf("at least one of --query or --page-id is required")
	}

	params := url.Values{}
	params.Set("fields", searchFields)
	params.Set("ad_type", searchAdType)
	params.Set("ad_active_status", searchStatus)

	// Countries as JSON array: ["US","DE"]
	countriesJSON := toJSONArray(searchCountries)
	params.Set("ad_reached_countries", countriesJSON)

	if searchQuery != "" {
		params.Set("search_terms", searchQuery)
	}

	if len(searchPageIDs) > 0 {
		params.Set("search_page_ids", toJSONArray(searchPageIDs))
	}

	if searchDateMin != "" {
		params.Set("ad_delivery_date_min", searchDateMin)
	}
	if searchDateMax != "" {
		params.Set("ad_delivery_date_max", searchDateMax)
	}

	if len(searchPlatforms) > 0 {
		params.Set("publisher_platforms", toJSONArray(searchPlatforms))
	}

	if len(searchLanguages) > 0 {
		params.Set("languages", toJSONArray(searchLanguages))
	}

	if searchMediaType != "" {
		params.Set("ad_creative_media_type", searchMediaType)
	}

	items, err := client.SearchAds(params, searchLimit)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		if output.IsJSON(cmd) {
			fmt.Println("[]")
			return nil
		}
		fmt.Println("no ads found")
		return nil
	}

	if output.IsJSON(cmd) {
		// Wrap in array for clean JSON output
		var raw []json.RawMessage
		raw = append(raw, items...)
		return output.PrintJSON(raw, output.IsPretty(cmd))
	}

	// Parse for table display
	ads := make([]api.AdArchiveRecord, 0, len(items))
	for _, raw := range items {
		var a api.AdArchiveRecord
		if err := json.Unmarshal(raw, &a); err != nil {
			return fmt.Errorf("parsing ad: %w", err)
		}
		ads = append(ads, a)
	}

	printAdsTable(ads)
	fmt.Printf("\n%d ad(s) returned\n", len(ads))
	return nil
}

func printAdsTable(ads []api.AdArchiveRecord) {
	headers := []string{"ID", "PAGE", "STARTED", "STATUS", "SPEND", "PLATFORMS", "BODY"}
	rows := make([][]string, len(ads))
	for i, a := range ads {
		status := "inactive"
		if a.AdDeliveryStopTime == "" {
			status = "active"
		}

		body := "-"
		if len(a.AdCreativeBodies) > 0 {
			body = output.Truncate(a.AdCreativeBodies[0], 50)
		} else if len(a.AdCreativeLinkTitles) > 0 {
			body = output.Truncate(a.AdCreativeLinkTitles[0], 50)
		}

		platforms := output.JoinStrings(a.PublisherPlatforms, ", ")

		spend := "-"
		if a.Spend != nil {
			spend = a.Spend.String()
			if a.Currency != "" {
				spend += " " + a.Currency
			}
		}

		rows[i] = []string{
			a.ID,
			output.Truncate(a.PageName, 25),
			output.FormatTime(a.AdDeliveryStartTime),
			status,
			spend,
			output.Truncate(platforms, 20),
			body,
		}
	}
	output.PrintTable(headers, rows)
}

// toJSONArray converts a slice of strings into a JSON array string, e.g. `["US","DE"]`.
func toJSONArray(ss []string) string {
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = strconv.Quote(s)
	}
	return "[" + strings.Join(quoted, ",") + "]"
}
