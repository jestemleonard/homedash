# Integration Guide

This directory contains YAML files that define how homedash connects to services. Each file describes one service — how to detect it, authenticate with its API, what data to fetch, and how to display it.

No Go code changes are needed to add or modify an integration.

## File structure

Every integration YAML has these top-level sections:

```yaml
# Metadata
id: myservice              # Must match the key in config.yaml services section
name: My Service           # Display name in the UI
icon: myservice            # Icon name from homarr-labs/dashboard-icons (see below)
color: "#hex"              # Brand color for the widget header

# How to find the service in Docker
detection:
  images: [...]            # Docker image patterns to match
  default_port: 8080       # Port to use when constructing the URL
  config_discovery:        # Optional: extract API key from container filesystem
    config_path: /config/config.xml
    format: xml            # xml, json, or yaml
    key: ApiKey            # Field name to extract (yaml supports dot paths: auth.apikey)

# How to authenticate with the API
auth:
  type: api_key            # none, api_key, or custom
  api_key:
    location: header       # header or query
    name: X-Api-Key        # Header/query parameter name

# API endpoints to poll
api:
  base_path: /api/v3       # Prepended to all endpoint paths
  endpoints:
    endpoint_name:
      path: /series
      method: GET
      interval: 300s       # How often to poll
      params: {}           # Query parameters (supports {{.api_key}}, {{.today}})
      mapping:             # How to extract data from the response
        field_name: $.json.path           # JSONPath (starts with $)
        other_field:
          expr: "length($)"              # Expression (expr-lang)

# What to show in the UI
widgets:
  - id: library
    name: Library
    endpoint: endpoint_name
    fields:
      field_name: {label: "Display Name", display: number}
```

## Adding a new integration

### 1. Research the service API

Before writing the YAML, you need to know:

- **API documentation**: Find the service's API docs. Most self-hosted apps document their API. Common places to look:
  - The project's GitHub wiki or docs site
  - `<service-url>/swagger` or `<service-url>/api-docs` (many services have built-in Swagger/OpenAPI docs)
  - The project's README or source code

- **Authentication method**: How does the API expect credentials? Common patterns:
  - API key in a header (e.g. Sonarr uses `X-Api-Key` header)
  - API key as a query parameter (e.g. Jellyfin uses `?api_key=`)
  - No auth required

- **Useful endpoints**: What data do you want to show? Look for endpoints that return stats, counts, or status information.

- **Docker images**: What Docker images does the service publish? Check Docker Hub or the project's install docs. Include the official image and popular alternatives (linuxserver, hotio).

- **Default port**: What port does the service listen on inside the container?

- **Config file location**: If the service stores its API key in a config file inside the container, note the path and format. This enables automatic API key extraction during Docker discovery.

### 2. Create the integration YAML

Create `integrations/<service>.yaml` using the structure above.

Use an existing file as a starting point:
- **Sonarr** (`sonarr.yaml`) — full example with config discovery and expressions
- **Jellyfin** (`jellyfin.yaml`) — simple example, query-param auth, no config discovery
- **Bazarr** (`bazarr.yaml`) — YAML config discovery with dot-path key

### 3. Add the config entry

Add an entry to `config.yaml` under `services:` with the same key as your integration's `id`:

```yaml
services:
  myservice:
    url: ${HOMEDASH_MYSERVICE_URL:-}
    api_key: ${HOMEDASH_MYSERVICE_API_KEY:-}
    external_url: ${HOMEDASH_MYSERVICE_EXTERNAL_URL:-}
```

This lets users configure the service URL and API key via environment variables, or rely on auto-discovery.

### 4. Add env vars to docker-compose.yml

Add the corresponding environment variables to `docker-compose.yml`, commented out for optional services:

```yaml
environment:
  # My Service
  # - HOMEDASH_MYSERVICE_URL=http://myservice:8080
  # - HOMEDASH_MYSERVICE_API_KEY=your-api-key
```

### 5. Update the README

If the service has any setup quirks (like needing a manually-generated API key), document it in the main `README.md` under the Services section.

## Data extraction reference

Endpoint mappings support two modes for extracting data from JSON API responses:

### JSONPath

String values starting with `$` are evaluated as JSONPath expressions (using the [ojg](https://github.com/ohler55/ojg) library):

```yaml
mapping:
  name: $.name                          # Simple field access
  first_type: $[0].type                 # Array index + field
  enabled: $[?@.enable == true]         # Filter expression
  count: $.MediaContainer.size          # Nested field
```

### Expressions

Map values with an `expr` key are evaluated using [expr-lang](https://github.com/expr-lang/expr). Two special functions are available that accept JSONPath inside them:

```yaml
mapping:
  total:
    expr: "length($)"                             # Count array items
  monitored:
    expr: "length($[?@.monitored == true])"       # Count filtered items
  episodes:
    expr: "sum($[*].statistics.episodeFileCount)"  # Sum a nested field
  status:
    expr: "length($.data) == 0 ? 'ok' : 'warning'" # Conditional
```

### Array iteration

Keys ending with `[]` iterate over an array, extracting fields from each item:

```yaml
mapping:
  items[]:
    name: $.name
    status: $.status
```

## Icons

Service icons are loaded from the [homarr-labs/dashboard-icons](https://github.com/homarr-labs/dashboard-icons) repository via CDN. The `icon` field in the YAML should match a filename (without `.svg`) from their `svg/` directory. For example, `icon: sonarr` loads `sonarr.svg`.

Browse available icons at: https://github.com/homarr-labs/dashboard-icons/tree/main/svg

## Tips

- **Keep intervals reasonable**: Don't poll faster than needed. 60s for health, 120-300s for data is typical.
- **Test incrementally**: Run homedash with `docker compose up -d --build` and check the logs. Failed endpoints log warnings but don't crash the app.
- **Field order matters**: Widget fields are displayed in the order they appear in the YAML. The engine uses a custom ordered unmarshaler to preserve this.
