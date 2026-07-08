# Swagger UI for HyperFleet API

This directory contains a standalone Swagger UI for browsing the HyperFleet API documentation.

## Usage

### Option 1: Open Directly (Simple)

Just open `index.html` in your browser:

```bash
open swagger-ui/index.html
# or
firefox swagger-ui/index.html
# or
google-chrome swagger-ui/index.html
```

**Note:** Due to CORS restrictions, you may need to serve via HTTP (see Option 2).

### Option 2: Serve via HTTP (Recommended)

Serve both the Swagger UI and OpenAPI spec together:

```bash
# Using Python (built-in)
python3 -m http.server 8080

# Using Go
go run -mod=mod github.com/httpwink/httpwink@latest

# Using Node.js
npx http-server -p 8080
```

Then open http://localhost:8080/swagger-ui/

### Option 3: Use the Makefile

```bash
make serve-swagger-ui
```

This starts a local server and opens the Swagger UI in your browser.

## What You'll See

The Swagger UI provides:
- **Interactive API documentation** - Browse all HyperFleet types
- **Schema details** - See all fields, types, and descriptions
- **Model explorer** - Expand/collapse type definitions
- **Filter/search** - Quickly find specific types
- **Try it out** - (disabled by default) Test API endpoints

## Regenerating

The Swagger UI reads from `../openapi/openapi.json`. To update:

```bash
make generate-openapi
```

Then refresh the Swagger UI in your browser.
