package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vincentmaurin/meta-ad-library-cli/internal/api"
	"github.com/vincentmaurin/meta-ad-library-cli/internal/output"
)

const adDetailFields = "id,ad_creation_time,ad_delivery_start_time,ad_delivery_stop_time," +
	"ad_creative_bodies,ad_creative_image_urls,ad_creative_link_captions," +
	"ad_creative_link_descriptions,ad_creative_link_titles," +
	"ad_snapshot_url,page_id,page_name,publisher_platforms,languages," +
	"spend,impressions,currency,bylines," +
	"region_distribution,demographic_distribution"

var adCmd = &cobra.Command{
	Use:   "ad",
	Short: "Get details about a specific ad",
}

var adGetCmd = &cobra.Command{
	Use:   "get <ad_archive_id>",
	Short: "Get detailed info for a specific ad by its archive ID",
	Long: `Fetches details for a single ad by its archive ID from the Ad Library.

The ad archive ID can be found in search results (the "id" field) or in the
ad_snapshot_url URL parameter.

Examples:
  meta-adlib ad get 123456789012345
  meta-adlib ad get 123456789012345 --json`,
	Args: cobra.ExactArgs(1),
	RunE: runAdGet,
}

func init() {
	adCmd.AddCommand(adGetCmd)
	rootCmd.AddCommand(adCmd)
}

func runAdGet(cmd *cobra.Command, args []string) error {
	id := args[0]

	params := url.Values{}
	params.Set("fields", adDetailFields)

	body, err := client.Get("/"+id, params)
	if err != nil {
		return err
	}

	var a api.AdArchiveRecord
	if err := json.Unmarshal(body, &a); err != nil {
		return fmt.Errorf("parsing ad: %w", err)
	}

	if output.IsJSON(cmd) {
		return output.PrintJSON(json.RawMessage(body), output.IsPretty(cmd))
	}

	printAdDetail(a)
	return nil
}

func printAdDetail(a api.AdArchiveRecord) {
	status := "inactive"
	if a.AdDeliveryStopTime == "" {
		status = "active"
	}

	spend := "-"
	if a.Spend != nil {
		spend = a.Spend.String()
		if a.Currency != "" {
			spend += " " + a.Currency
		}
	}

	impr := "-"
	if a.Impressions != nil {
		impr = a.Impressions.String()
	}

	rows := [][]string{
		{"ID", a.ID},
		{"Page", a.PageName + " (ID: " + a.PageID + ")"},
		{"Status", status},
		{"Created", output.FormatTime(a.AdCreationTime)},
		{"Started", output.FormatTime(a.AdDeliveryStartTime)},
		{"Stopped", output.FormatTime(a.AdDeliveryStopTime)},
		{"Platforms", output.JoinStrings(a.PublisherPlatforms, ", ")},
		{"Languages", output.JoinStrings(a.Languages, ", ")},
		{"Bylines", a.Bylines},
		{"Spend (est.)", spend},
		{"Impressions (est.)", impr},
		{"Snapshot URL", a.AdSnapshotURL},
	}

	if len(a.AdCreativeBodies) > 0 {
		rows = append(rows, []string{"Body", strings.Join(a.AdCreativeBodies, " | ")})
	}
	if len(a.AdCreativeLinkTitles) > 0 {
		rows = append(rows, []string{"Link Title", strings.Join(a.AdCreativeLinkTitles, " | ")})
	}
	if len(a.AdCreativeLinkDescriptions) > 0 {
		rows = append(rows, []string{"Link Description", strings.Join(a.AdCreativeLinkDescriptions, " | ")})
	}
	if len(a.AdCreativeLinkCaptions) > 0 {
		rows = append(rows, []string{"Link Caption", strings.Join(a.AdCreativeLinkCaptions, " | ")})
	}
	if len(a.AdCreativeImageURLs) > 0 {
		rows = append(rows, []string{"Image URLs", strings.Join(a.AdCreativeImageURLs, "\n")})
	}

	output.PrintKeyValue(rows)

	if len(a.RegionDistribution) > 0 {
		fmt.Println("\nRegion Distribution:")
		for _, d := range a.RegionDistribution {
			fmt.Printf("  %-30s %.1f%%\n", d.Region, d.Percentage)
		}
	}

	if len(a.DemographicDistribution) > 0 {
		fmt.Println("\nDemographic Distribution:")
		for _, d := range a.DemographicDistribution {
			fmt.Printf("  %-5s %-10s %.1f%%\n", d.Gender, d.Age, d.Percentage)
		}
	}
}
