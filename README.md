# meta-adlib

CLI for the [Meta Ad Library API](https://www.facebook.com/ads/library/api) — search and explore public Meta ads programmatically.

Provides read-only access to the public Meta Ad Library: ads running on Facebook, Instagram, Messenger, Audience Network, and Threads.

**Available for:**
- All ads targeting the **European Union**
- **Political and issue ads** worldwide
- Ads in **Brazil** (limited scope)

---

## Quick start

```bash
# Authenticate (uses shared meta-auth store — recommended)
meta-auth login

# Or set a token directly
meta-adlib auth set-token EAABsbCS...

# Search ads
meta-adlib search --query "climate" --country FR
meta-adlib search --query "election" --country US --type POLITICAL_AND_ISSUE_ADS --status ACTIVE

# Browse a page's ads
meta-adlib page ads 123456789 --country DE

# Get details for one ad
meta-adlib ad get 123456789012345
```

---

## Installation

```bash
cd meta-ad-library-cli
go build -o meta-adlib .
mv meta-adlib /usr/local/bin/
```

---

## Authentication

Token resolution order (same as all Meta CLIs):

| Priority | Source | How to set |
|----------|--------|------------|
| 1 | `META_TOKEN` env var | `export META_TOKEN=EAABsb...` |
| 2 | Own config (`~/.config/meta-ad-library/config.json`) | `meta-adlib auth set-token` |
| 3 | Shared meta-auth config (`~/.config/meta-auth/config.json`) | `meta-auth login` ← recommended |

The Ad Library API does **not** require App credentials for basic public data — a simple user token with `public_profile` is sufficient.

---

## Commands

### Global flags

| Flag | Description |
|------|-------------|
| `--json` | Force JSON output |
| `--pretty` | Force pretty-printed JSON (implies `--json`) |

---

### `search`

Search the Ad Library via the `/ads_archive` endpoint.

**Requires:** at least one `--country` AND at least one of `--query` or `--page-id`.

```bash
meta-adlib search --query "gratuit" --country FR
meta-adlib search --query "election" --country US --type POLITICAL_AND_ISSUE_ADS --status ACTIVE
meta-adlib search --query "cars" --country FR --country DE --platform facebook --platform instagram
meta-adlib search --query "health" --country US --since 2024-01-01 --until 2024-12-31
meta-adlib search --page-id 123456789 --country DE --limit 100
meta-adlib search --query "shoes" --country US --json | jq '.[].page_name'
```

**Options:**

| Flag | Default | Description |
|------|---------|-------------|
| `--query` | | Search terms in ad creative text |
| `--country` | | Country code (ISO 3166, e.g. `FR`, `US`, `DE`). Repeatable. |
| `--page-id` | | Facebook Page ID(s) to filter. Repeatable. |
| `--type` | `ALL` | `ALL` or `POLITICAL_AND_ISSUE_ADS` |
| `--status` | `ALL` | `ALL` or `ACTIVE` |
| `--since` | | Min delivery start date (`YYYY-MM-DD`) |
| `--until` | | Max delivery start date (`YYYY-MM-DD`) |
| `--platform` | | Platform filter: `facebook`, `instagram`, `audience_network`, `messenger`, `threads`. Repeatable. |
| `--language` | | Language filter (ISO 639-1, e.g. `en`, `fr`). Repeatable. |
| `--media-type` | | `ALL`, `IMAGE`, `MEME`, `VIDEO`, `NONE` |
| `--limit` | `25` | Max results (0 = fetch all pages) |
| `--fields` | *(see below)* | Comma-separated fields to return |

**Default fields:** `id`, `ad_creation_time`, `ad_delivery_start_time`, `ad_delivery_stop_time`, `ad_creative_bodies`, `ad_creative_link_titles`, `ad_creative_link_captions`, `ad_snapshot_url`, `page_id`, `page_name`, `publisher_platforms`, `languages`, `spend`, `impressions`, `currency`

---

### `ad get <ad_archive_id>`

Get full details for a single ad by its archive ID (from search results or the `ad_snapshot_url` URL parameter).

```bash
meta-adlib ad get 123456789012345
meta-adlib ad get 123456789012345 --pretty
```

**Detail fields returned:** everything from search, plus `ad_creative_image_urls`, `ad_creative_link_descriptions`, `bylines`, `region_distribution`, `demographic_distribution`.

---

### `page ads <page_id>`

List all ads associated with a Facebook Page ID.

```bash
meta-adlib page ads 123456789 --country US
meta-adlib page ads 123456789 --country DE --status ACTIVE
meta-adlib page ads 123456789 --country US --type POLITICAL_AND_ISSUE_ADS --limit 200 --json
```

**Options:** same as `search` (minus `--query` / `--page-id`).

---

### auth (local-only auth management)

These commands manage a local token stored in `~/.config/meta-ad-library/config.json`. For shared auth across all Meta tools, use `meta-auth` instead.

#### `auth set-token <token>`
Save and validate a token. Auto-extends to long-lived (~60 days) if `META_APP_ID` / `META_APP_SECRET` are set.
- `--no-extend` — skip the upgrade

#### `auth extend-token <short_lived_token>`
Exchange a short-lived token for a long-lived one.
- `--save` — also save to local config

#### `auth refresh`
Re-exchange the stored token for a fresh 60-day token. Requires `META_APP_ID` / `META_APP_SECRET`.

#### `auth status`
Show current auth state, expiry, and days remaining.

#### `auth logout`
Remove local credentials.

---

## Output format

- **Terminal:** human-readable aligned table
- **Pipe / `--json`:** newline-delimited JSON array, suitable for `jq`
- **`--pretty`:** indented JSON

```bash
# Filter with jq
meta-adlib search --query "shoes" --country FR --json | jq '.[].page_name'

# Save to file
meta-adlib search --query "election" --country US --limit 0 --json > ads.json
```

---

## Notes

- **Spend and impressions** are estimated ranges (e.g. `1000–5000`), not exact figures — Meta policy.
- **`funding_entity`** field is deprecated since API v13 and not requested.
- **Pagination** is handled automatically — set `--limit 0` to fetch all results across all pages.
- **Rate limits:** the CLI warns to stderr if usage exceeds 75% of the API quota.
