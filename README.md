# rss-proxy

A simple HTTP proxy that fetches an RSS feed and filters items by keyword. Originally built for the [Enode status page](https://status.enode.com/slack.rss) Slack RSS feed, which publishes updates for all integrations regardless of which ones you actually use.

## Configuration

Copy `config.example.json` to `config.json` and edit it:

```json
{
  "upstream": "https://example.com/feed.rss",
  "profiles": {
    "vehicles": ["Tesla", "Volvo", "BMW"]
  }
}
```

- `upstream` — the RSS feed to proxy
- `profiles` — named groups of services to filter by

## Usage

```
GET /feed                              # full feed, no filtering
GET /feed?profile=vehicles             # items matching any service in the profile
GET /feed?services=Tesla,Volvo         # ad-hoc filter
GET /feed?profile=vehicles&services=Stripe  # combined
```

Matching is case-insensitive. An unknown profile returns 400.

## Running

```bash
# Local
go run .

# Docker
docker compose up -d
```

Environment variables: `PORT` (default `8080`), `CONFIG` (default `config.json`).
