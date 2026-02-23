package api

import "encoding/json"

// MetaError wraps a Meta API error response.
type MetaError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Subcode int    `json:"error_subcode"`
}

func (e *MetaError) Error() string {
	if e.Subcode != 0 {
		return "meta api error " + itoa(e.Code) + " (subcode " + itoa(e.Subcode) + "): " + e.Message
	}
	return "meta api error " + itoa(e.Code) + ": " + e.Message
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	b := make([]byte, 0, 12)
	if n < 0 {
		b = append(b, '-')
		n = -n
	}
	tmp := make([]byte, 0, 12)
	for n > 0 {
		tmp = append(tmp, byte('0'+n%10))
		n /= 10
	}
	for i := len(tmp) - 1; i >= 0; i-- {
		b = append(b, tmp[i])
	}
	return string(b)
}

// Paging wraps the paging field in list responses.
type Paging struct {
	Cursors *struct {
		Before string `json:"before"`
		After  string `json:"after"`
	} `json:"cursors,omitempty"`
	Next     string `json:"next,omitempty"`
	Previous string `json:"previous,omitempty"`
}

// AdArchiveRecord is an ad returned by the /ads_archive endpoint.
type AdArchiveRecord struct {
	ID                      string          `json:"id"`
	AdCreationTime          string          `json:"ad_creation_time,omitempty"`
	AdCreativeBodies        []string        `json:"ad_creative_bodies,omitempty"`
	AdCreativeImageURLs     []string        `json:"ad_creative_image_urls,omitempty"`
	AdCreativeLinkCaptions  []string        `json:"ad_creative_link_captions,omitempty"`
	AdCreativeLinkDescriptions []string     `json:"ad_creative_link_descriptions,omitempty"`
	AdCreativeLinkTitles    []string        `json:"ad_creative_link_titles,omitempty"`
	AdDeliveryStartTime     string          `json:"ad_delivery_start_time,omitempty"`
	AdDeliveryStopTime      string          `json:"ad_delivery_stop_time,omitempty"`
	AdSnapshotURL           string          `json:"ad_snapshot_url,omitempty"`
	Currency                string          `json:"currency,omitempty"`
	// Spend is an estimated range; Meta returns {"lower_bound":"N","upper_bound":"N"}
	Spend                   *RangeValue     `json:"spend,omitempty"`
	// Impressions is similarly an estimated range
	Impressions             *RangeValue     `json:"impressions,omitempty"`
	// Languages contains ISO 639-1 codes
	Languages               []string        `json:"languages,omitempty"`
	// Distribution percentages by region/demographic
	RegionDistribution      []Distribution  `json:"region_distribution,omitempty"`
	DemographicDistribution []DemoDistribution `json:"demographic_distribution,omitempty"`
	// For political/issue ads
	FundingEntity           string          `json:"funding_entity,omitempty"`
	// Page info
	PageID                  string          `json:"page_id,omitempty"`
	PageName                string          `json:"page_name,omitempty"`
	// Bylines for EU Transparency (political ads)
	Bylines                 string          `json:"bylines,omitempty"`
	// Publisher platforms
	PublisherPlatforms      []string        `json:"publisher_platforms,omitempty"`
	// Additional raw data for pass-through
	Extra                   json.RawMessage `json:"-"`
}

// RangeValue represents Meta's estimated ranges (spend, impressions).
type RangeValue struct {
	LowerBound string `json:"lower_bound"`
	UpperBound string `json:"upper_bound"`
}

func (r *RangeValue) String() string {
	if r == nil {
		return "-"
	}
	if r.LowerBound == r.UpperBound {
		return r.LowerBound
	}
	return r.LowerBound + "–" + r.UpperBound
}

// Distribution represents a percentage breakdown by region.
type Distribution struct {
	Region     string  `json:"region"`
	Percentage float64 `json:"percentage"`
}

// DemoDistribution represents a percentage breakdown by age/gender.
type DemoDistribution struct {
	Age        string  `json:"age"`
	Gender     string  `json:"gender"`
	Percentage float64 `json:"percentage"`
}

// User is returned by GET /me.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
}
