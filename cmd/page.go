package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/vincentmaurin/meta-ad-library-cli/internal/api"
	"github.com/vincentmaurin/meta-ad-library-cli/internal/output"
)

var (
	pageCountries []string
	pageAdType    string
	pageStatus    string
	pageLimit     int
	pageDateMin   string
	pageDateMax   string
)

var pageCmd = &cobra.Command{
	Use:   "page",
	Short: "Browse ads by Facebook Page",
}

var pageAdsCmd = &cobra.Command{
	Use:   "ads <page_id>",
	Short: "List all ads for a specific Facebook Page",
	Long: `Fetches all ads associated with a given Facebook Page ID.

This is equivalent to searching by --page-id but as a dedicated sub-command
with a friendlier interface for page-focused research.

Examples:
  meta-adlib page ads 123456789 --country US
  meta-adlib page ads 123456789 --country DE --status ACTIVE
  meta-adlib page ads 123456789 --country US --type POLITICAL_AND_ISSUE_ADS --limit 100 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runPageAds,
}

func init() {
	pageAdsCmd.Flags().StringArrayVar(&pageCountries, "country", nil, "Country code(s) (ISO 3166). Repeatable.")
	pageAdsCmd.Flags().StringVar(&pageAdType, "type", "ALL", "Ad type: ALL or POLITICAL_AND_ISSUE_ADS")
	pageAdsCmd.Flags().StringVar(&pageStatus, "status", "ALL", "Ad active status: ALL or ACTIVE")
	pageAdsCmd.Flags().IntVar(&pageLimit, "limit", 25, "Maximum number of results (0 = fetch all pages)")
	pageAdsCmd.Flags().StringVar(&pageDateMin, "since", "", "Minimum delivery start date (YYYY-MM-DD)")
	pageAdsCmd.Flags().StringVar(&pageDateMax, "until", "", "Maximum delivery start date (YYYY-MM-DD)")

	pageCmd.AddCommand(pageAdsCmd)
	rootCmd.AddCommand(pageCmd)
}

func runPageAds(cmd *cobra.Command, args []string) error {
	pageID := args[0]

	if len(pageCountries) == 0 {
		return fmt.Errorf("at least one --country is required (e.g. --country US)")
	}

	params := url.Values{}
	params.Set("fields", defaultFields)
	params.Set("ad_type", pageAdType)
	params.Set("ad_active_status", pageStatus)
	params.Set("ad_reached_countries", toJSONArray(pageCountries))
	params.Set("search_page_ids", toJSONArray([]string{pageID}))

	if pageDateMin != "" {
		params.Set("ad_delivery_date_min", pageDateMin)
	}
	if pageDateMax != "" {
		params.Set("ad_delivery_date_max", pageDateMax)
	}

	items, err := client.SearchAds(params, pageLimit)
	if err != nil {
		return err
	}

	if len(items) == 0 {
		if output.IsJSON(cmd) {
			fmt.Println("[]")
			return nil
		}
		fmt.Printf("no ads found for page %s\n", pageID)
		return nil
	}

	if output.IsJSON(cmd) {
		var raw []json.RawMessage
		raw = append(raw, items...)
		return output.PrintJSON(raw, output.IsPretty(cmd))
	}

	ads := make([]api.AdArchiveRecord, 0, len(items))
	for _, raw := range items {
		var a api.AdArchiveRecord
		if err := json.Unmarshal(raw, &a); err != nil {
			return fmt.Errorf("parsing ad: %w", err)
		}
		ads = append(ads, a)
	}

	printAdsTable(ads)
	fmt.Printf("\n%d ad(s) for page %s\n", len(ads), pageID)
	return nil
}
