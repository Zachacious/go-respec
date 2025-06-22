![Go Reference](images/respec.jpg)

# RESPEC: Generate OpenAPI v3 Specifications from Go Code

respec generates OpenAPI v3 specifications from Go source code by statically analyzing your project. It works without code annotations by inferring API structures directly from your router definitions and handler implementations.

[![Go Report Card](https://goreportcard.com/badge/github.com/Zachacious/go-respec)](https://goreportcard.com/report/github.com/Zachacious/go-respec)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Features

- **Annotation-Free**: Analyzes Go code directly without requiring "magic comments" in your business logic
- **Refactoring-Aware**: Uses the Go type system, so renaming handlers or structs is automatically reflected in the generated spec
- **Framework-Agnostic**: Works with Chi and Gin out of the box, configurable for any routing library via `.respec.yaml`
- **Layered Metadata System**:
  - **Automatic Inference**: Discovers routes, parameters (path, query, header), and request/response schemas
  - **Doc Comments**: Uses standard Go doc comments to populate descriptions, summaries, and tags
  - **Fluent Builder API**: Type-safe Go library for explicit overrides and complex metadata

## Installation

### Using go install (recommended)

```bash
go install github.com/Zachacious/go-respec/cmd/respec@latest
```

### From GitHub Releases

1. Download the appropriate binary from the [Releases page](https://github.com/Zachacious/go-respec/releases)
2. Place the `respec` binary in a directory within your system's PATH

## Quick Start

1. Navigate to your Go project's root directory

2. (Optional) Create a `.respec.yaml` configuration file:

   ```yaml
   info:
     title: "My Awesome API"
     version: "1.0.0"
     description: "This API manages widgets and gadgets."
   securitySchemes:
     BearerAuth:
       type: http
       scheme: bearer
       bearerFormat: JWT
   ```

3. Generate the OpenAPI specification:

   ```bash
   respec generate ./... --output openapi.yaml
   ```

## Usage

respec uses a three-tiered metadata system with clear priority ordering:

**Priority Order:** Fluent Builder API > Doc Comments > Static Inference

### Level 3: Static Inference

Baseline behavior that automatically discovers:

- **Routes & Groups**: Nested routing patterns
- **Parameters**: Path (`/users/{id}`), query (`c.Query("page")`), and header (`r.Header.Get("...")`) parameters
- **Request/Response Bodies**: Schemas from Go structs used in functions like `c.BindJSON()` and `c.JSON()`

### Level 2: Doc Comments

Add standard Go doc comments to your HTTP handlers:

```go
// GetUser retrieves a specific user by their unique ID.
// This endpoint is part of the core user management functionality.
// @summary Get a User by ID
// @tags Users
func GetUser(w http.ResponseWriter, r *http.Request) {
    // ...
}
```

### Level 1: Fluent Builder API

Import the library and wrap route definitions for maximum control:

```go
import "github.com/Zachacious/go-respec/respec"

r := chi.NewRouter()

respec.Route(r.Post("/users", handlers.CreateUser)).
    Summary("Create a new User").
    Description("Creates a new user and returns the created user object.").
    Tag("Users", "Admin").
    Security("BearerAuth").
    OverrideParam("id", func(p *openapi3.Parameter) {
        p.Description = "The user's unique identifier (UUID)"
        p.Schema.Value.Format = "uuid"
    })
```

## Configuration

Configure respec for custom routing frameworks in `.respec.yaml`:

```yaml
info:
  title: "My Custom API"
  version: "v1.5.0"

securitySchemes:
  APIKey:
    type: apiKey
    in: header
    name: X-API-KEY

# Configure custom router recognition
routerDefinitions:
  - type: "*github.com/my-corp/custom-router.Router"
    endpointMethods: ["Handle"]
    groupMethods: ["RouteGroup"]
    middlewareWrapperMethods: ["With"]
```

## CLI Reference

```bash
# Generate for current directory
respec generate .

# Generate for specific packages
respec generate ./api ./handlers

# Custom output file
respec generate . --output api-spec.yaml

# Show help
respec --help
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
