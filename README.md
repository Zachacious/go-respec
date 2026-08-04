<h1 align="center">go-respec</h1>

<p align="center"><strong>Generate OpenAPI 3.1 specs from Go source code — no annotations, no code generation, no wrappers.</strong></p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/Zachacious/go-respec"><img src="https://pkg.go.dev/badge/github.com/Zachacious/go-respec.svg" alt="Go Reference"></a>
  <a href="https://github.com/Zachacious/go-respec/releases"><img src="https://img.shields.io/github/v/release/Zachacious/go-respec" alt="Latest release"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/Zachacious/go-respec" alt="Go version">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT license"></a>
</p>

![go-respec](images/respec2.png)

## What it is

`respec` is a CLI tool that statically analyzes a Go project and writes an OpenAPI 3.1 specification. It reads your router setup, follows the handlers, and derives paths, parameters, request bodies, responses, and component schemas from the types already in your code.

Nothing is required in your source to make it work. When inference isn't enough, a small fluent API lets you override any part of an operation in plain Go — no comment DSL.

## Why

Most OpenAPI tooling for Go asks you to describe your API twice: once in code, once in annotation comments that drift out of date, or through generated stubs you have to build around.

`respec` takes the source as the single description. If the routes are in the code, they're in the spec.

## Status

Stable. The `respec` package API is covered by semantic versioning: it will not
change incompatibly before v2.

Behaviour is pinned by golden-file tests over real router setups for chi, gin,
echo, and `net/http` — see `internal/testsuite/testdata`. Anything respec cannot
resolve is reported with a file and line rather than quietly left out. Known gaps
are listed under [Known limitations](#known-limitations). Issues and pull
requests are welcome.

The `respec` package you import into your service has no dependencies at all —
it compiles to a pass-through and costs nothing at run time.

## Installation

With `go install`:

```bash
go install github.com/Zachacious/go-respec/cmd/respec@latest
```

Or download a binary for your platform from the [releases page](https://github.com/Zachacious/go-respec/releases).

## Quick start

From your project root:

```bash
cd /path/to/your/project

# optional: write a starter .respec.yaml
respec init

# analyze the current directory, write openapi.yaml
respec . -o openapi.yaml
```

`respec init` writes `.respec.yaml` into the current directory and aborts if one already exists. The config is read from the root of the project being analyzed, so run both commands from the same place.

## How it resolves values

Three layers, highest priority first. Each layer only fills in what the layer above it left alone.

**1. Metadata API.** Explicit overrides written in Go with `respec.Handler()` and `respec.Meta()`. See [Metadata API](#metadata-api).

**2. Doc comments.** A handler's doc comment supplies its summary and description. The first line becomes the summary and the rest becomes the description, unless an `@summary <text>` line says otherwise. Because Go convention is to open a comment with the identifier it documents, the leading name is dropped: `// ListBooks returns the catalogue.` becomes the summary `Returns the catalogue.` An `@tags a, b` line sets tags, outranking the enclosing group but yielding to an explicit `.Tag()`. These are ordinary Go doc comments, not a required annotation format.

**3. Inference.** Everything else is read from the code:

| What | Where it comes from |
| --- | --- |
| Paths and route tree | Router method calls (`Get`, `Post`, `Route`, `Group`, `With`, `Use`, ...) |
| Path parameters | Placeholders in the route pattern, in whichever syntax the router uses; the type is narrowed to integer or number if the handler parses it as one |
| Header parameters | `r.Header.Get("...")` calls in the handler body |
| Query parameters | `r.URL.Query().Get("...")` and any configured `queryParameter` pattern |
| Request body | `json.NewDecoder(...).Decode(&req)` and any configured binder, such as `gin.Context.ShouldBindJSON` |
| Responses | `w.WriteHeader(code)` paired with `json.NewEncoder(w).Encode(v)`, plus any configured response helper |
| Component schemas | The Go types themselves. `json` tags set property names and `json:"-"` is skipped; embedded structs are inlined as `encoding/json` inlines them; `time.Time`, `uuid.UUID`, and `[]byte` get their proper string formats; recursive and mutually recursive types resolve to `$ref`s. Same-named types in different packages, and separate instantiations of a generic type, are qualified rather than merged. |
| Required and nullable | The `json` tag, following what `encoding/json` actually emits. A field without `omitempty` is always present, so it is `required`; a pointer without `omitempty` encodes as `null` when nil, so it is also nullable. A field with `omitempty` is simply absent when empty, so it is neither. |
| Security | Calls inside middleware bodies that match a `securityPatterns` entry |
| Summary | The handler function name, when no doc comment or override supplies one. A handler respec cannot name is left without a summary rather than given a placeholder. |

## Metadata API

Import the package:

```go
import "github.com/Zachacious/go-respec/respec"
```

These calls are read statically at analysis time — they cost nothing at runtime, and their arguments must be literals or constants so the analyzer can resolve them.

### Per-route: `respec.Handler()`

Wrap a handler, chain the overrides, and call `.Unwrap()` to hand the original handler back to the router:

```go
// before
r.With(mw.Authenticator).Post("/users", userHandlers.Create)

// after
r.With(mw.Authenticator).Post("/users",
    respec.Handler(userHandlers.Create).
        Tag("User Management").
        Summary("Create a new system user").
        Security("BearerAuth").
        Unwrap(),
)
```

`.Unwrap()` returns the handler unchanged and must end every chain.

### Per-group: `respec.Meta()`

Call `respec.Meta()` with the group's router variable to apply metadata to every route in that group and its children:

```go
r.Route("/admin", func(r chi.Router) {
    respec.Meta(r).
        Tag("Admin").
        Security("AdminSecurity")

    r.Use(mw.AdminOnly)
    r.Get("/users", respec.Handler(admin.ListUsers).Unwrap())
    r.Post("/users", respec.Handler(admin.CreateUser).Unwrap())
})
```

### Methods

| Method | Description | Applies to |
| --- | --- | --- |
| `.Summary(string)` | Sets the operation summary. | Handler |
| `.Description(string)` | Sets the longer operation description. | Handler |
| `.Tag(...string)` | On a handler, replaces every inherited tag. On a group, adds tags to all routes beneath it. | Handler, Meta |
| `.Security(...string)` | On a handler, replaces all inherited security. On a group, applies to all routes beneath it. All arguments are read. | Handler, Meta |
| `.Public()` | Clears inherited security. Use for login or health routes inside an authenticated group. | Handler, Meta |
| `.RequestBody(obj)` | Sets the request body to a schema generated from `obj`. | Handler |
| `.Response(code, ...ResponseOption)` | Declares a response. Compose `respec.Schema(v)` and `respec.Desc(s)` to set both at once. | Handler |
| `.AddResponse(code, content)` | Shorthand: a value becomes the schema, a string becomes the description. | Handler |
| `.QueryParam(name, ...ParamOption)` | Declares a query parameter. | Handler |
| `.PathParam(name, ...ParamOption)` | Declares a path parameter (always required). | Handler |
| `.HeaderParam(name, ...ParamOption)` | Declares a header parameter. | Handler |
| `.CookieParam(name, ...ParamOption)` | Declares a cookie parameter. | Handler |
| `.ResponseHeader(code, name, ...HeaderOption)` | Adds a header to a response code. | Handler |
| `.OperationID(string)` | Sets `operationId`. | Handler |
| `.Deprecated()` | Marks the operation or group deprecated. | Handler, Meta |
| `.Hide()` | Omits the operation from the spec entirely. | Handler |
| `.ExternalDocs(url, desc)` | Links external documentation. | Handler |
| `.AddServer(url, desc)` | Adds an operation-level server URL. | Handler |
| `.Extensions(map[string]any)` | Adds specification extensions, conventionally prefixed `x-`. | Handler |
| `.Unwrap()` | Returns the original handler. Required as the last call in a chain. | Handler |

Options:

| Option | Applies to | Description |
| --- | --- | --- |
| `respec.Schema(v)` | `.Response` | Response body schema, generated from `v`'s type. |
| `respec.Desc(s)` | `.Response` | Response description. |
| `respec.Doc(s)` | parameters | Parameter description. |
| `respec.Type(s)` | parameters | `string` (default), `integer`, `number`, or `boolean`. |
| `respec.Required()` | parameters | Marks the parameter required. |
| `respec.Example(s)` | parameters | Example value. |
| `respec.DeprecatedParam()` | parameters | Marks the parameter deprecated. |
| `respec.HeaderDoc(s)` | `.ResponseHeader` | Header description. |
| `respec.HeaderType(s)` | `.ResponseHeader` | Header primitive type. |

### Full example

```go
type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

type UserResponse struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

type ErrorResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

r.Post("/users",
    respec.Handler(userHandlers.Create).
        Summary("Create a new system user").
        Description("Creates a user account and returns the created user.").
        Tag("Users", "Write Ops").
        Security("BearerAuth", "ApiKeyAuth").
        OperationID("users-create").
        Extensions(map[string]any{
            "x-custom-extension":  "value",
            "x-another-extension": true,
        }).

        // request and response bodies
        RequestBody(CreateUserRequest{}).
        Response(201, respec.Schema(UserResponse{}), respec.Desc("User created")).
        Response(409, respec.Schema(ErrorResponse{}), respec.Desc("Email already registered")).
        Response(400, respec.Desc("Invalid request payload provided.")).

        // parameters and response headers
        QueryParam("source", respec.Doc("The source of the registration."), respec.Example("web")).
        ResponseHeader(201, "X-RateLimit-Remaining",
            respec.HeaderDoc("Requests remaining for this client."),
            respec.HeaderType("integer")).

        // external references
        ExternalDocs("https://docs.example.com/users/create", "API usage guide").
        AddServer("https://users.api.example.com", "Users API server").

        Unwrap(),
)
```

Handlers do not have to be written inline. respec follows a builder chain through
a variable, through a factory function, and into an anonymous function, so all of
these produce the same result:

```go
// factory method
func (h *UserHandler) CreateSpec() http.HandlerFunc {
    return respec.Handler(h.Create).Summary("Create a user").Unwrap()
}
r.Post("/users", h.CreateSpec())

// local variable
create := respec.Handler(h.Create).Summary("Create a user").Unwrap()
r.Post("/users", create)

// anonymous handler
r.Get("/health", respec.Handler(func(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(Status{})
}).Summary("Health check").Unwrap())
```

If a chain never reaches a route, respec says so rather than silently omitting it:

```
⚠ routes/users.go:42:9: respec.Handler(Create) metadata was never attached to a
  route; check that the value returned by .Unwrap() is registered on a router
  respec is tracking
```

## CLI

```bash
respec [path] [flags]
```

`path` is the directory to analyze and defaults to the current directory.

| Flag | Description |
| --- | --- |
| `-o, --output` | Output file. `.json` selects JSON, anything else YAML. Use `-` for stdout. Defaults to `openapi.yaml`. |
| `-q, --quiet` | Suppress progress and diagnostics. |
| `-v, --verbose` | Also show informational diagnostics. |
| `--strict` | Exit non-zero if anything could not be resolved. Use this in CI. |

Progress and diagnostics go to stderr, so `respec . -o -` pipes a clean spec to
stdout.

Subcommands: `respec init` writes a starter `.respec.yaml`, `respec validate <file>`
checks a spec against OpenAPI 3.1, and `respec version` prints build info.

## Upgrading to v1

Two builder methods changed shape. Both fail at compile time, so nothing breaks
silently, and everything else — `Summary`, `Description`, `Tag`, `Security`,
`RequestBody`, `AddResponse`, `OperationID`, `Deprecate`, `AddServer`,
`ExternalDocs`, `Extensions`, `Unwrap` — is unchanged.

| Before | Now |
| --- | --- |
| `.AddParameter("query", "q", "Search", true, false)` | `.QueryParam("q", respec.Doc("Search"), respec.Required())` |
| `.ResponseHeader(201, "X-Rate", "Remaining")` | `.ResponseHeader(201, "X-Rate", respec.HeaderDoc("Remaining"))` |

`AddParameter`'s five positional arguments, two of them unlabelled booleans, were
the least readable thing in the API, and a parameter could not declare a type or
an example. `ResponseHeader` emitted a header with no schema, which strict
validators — including `respec validate` — reject.

Also removed: the analyzer-facing types the package used to export
(`HandlerMetadata`, `ParameterOverride`, `ResponseOverride`, and friends). They
described results of static analysis, never anything a running program used, and
two of them carried `go/ast` values — which meant every service importing
`respec` compiled `go/ast`, `go/scanner`, and `go/token` into its binary. The
package now has no dependencies at all.

## Configuration

`.respec.yaml` lives at the root of the project being analyzed. Every section is
optional — respec runs with no config at all. Generate a commented starter with
`respec init`.

### Spec metadata

```yaml
info:
  title: "Bookstore API"
  version: "2.1.0"
servers:
  - url: "https://api.example.com"
    description: "Production"
securitySchemes:
  BearerAuth:
    type: http
    scheme: bearer
    bearerFormat: JWT
  ApiKeyAuth:
    type: apiKey
    in: header
    name: X-API-Key
```

`securitySchemes` is passed through in full, including `in`/`name` for `apiKey`,
`openIdConnectUrl`, and oauth2 `flows`. The names here are what you pass to
`.Security("BearerAuth")` in code.

### Teaching respec your helpers

If your handlers go through project-specific request or response helpers, name
them so their arguments can be read. Standard library, gin, and echo patterns
are built in.

```yaml
handlerPatterns:
  requestBody:
    - functionPath: "myapp/internal/httputil.Bind"
      argIndex: 0
  responseBody:
    - functionPath: "myapp/internal/httputil.JSON"
      statusCodeIndex: 1     # omit if the helper has no status argument
      dataIndex: 2
      descriptionIndex: 3    # omit if it has no description argument
  queryParameter:
    - functionPath: "net/url.Values.Get"
      nameIndex: 0
  headerParameter:
    - functionPath: "net/http.Header.Get"
      nameIndex: 0

securityPatterns:
  - functionPath: "*myapp/internal/auth.Service.Validate"
    schemeName: "BearerAuth"
```

Writing a `functionPath`:

- Package-level functions are `<import path>.<Func>`.
- Methods are `<qualified receiver type>.<Method>`.
- Under `securityPatterns`, a pointer-receiver method needs a leading `*`
  (`*pkg.Service.Validate`). Under `handlerPatterns` it does not. This
  inconsistency is a bug, not a design decision.

### Describing a router

respec has no framework-specific code; the routers it knows are just entries in
this list. Add one for anything else. These fields are the whole model:

```yaml
routerDefinitions:
  - type: "github.com/example/router.Router"

    # Methods named after the HTTP verb: r.Get("/x", h)
    endpointMethods: ["Get", "Post", "Put", "Patch", "Delete"]

    # Methods opening a nested scope: r.Route("/users", func(r Router){...})
    groupMethods: ["Route", "Group"]

    # Methods attaching middleware: r.Use(mw) / r.With(mw)
    middlewareWrapperMethods: ["With", "Use"]

    # Methods grafting an existing sub-router under a prefix: r.Mount("/x", sub)
    mountMethods: ["Mount"]

    # "brace" for /users/{id}, "colon" for /users/:id. Colon paths are
    # rewritten to the brace form, which is all OpenAPI understands.
    pathParamStyle: "brace"

    # Registration calls whose arguments are not (path, handler).
    endpointPatterns:
      # r.Handle("GET", "/x", h) - verb in an argument
      - methods: ["Handle"]
        methodArg: 0
        pathArg: 1
        handlerArg: 2

      # mux.HandleFunc("GET /x", h) - verb inside the pattern string
      - methods: ["HandleFunc"]
        pathArg: 0
        handlerArg: 1
        methodInPath: true
        # Which verbs to document when the pattern names none. Such a pattern
        # matches all of them, but emitting five near-identical operations for
        # one static file tree is noise, so the default is a single GET.
        defaultMethods: ["GET"]
```

## Framework support

There is no framework-specific code in the analyzer. Routers are described
entirely by `routerDefinitions`, and the built-in defaults cover four of them.

| Framework | Support |
| --- | --- |
| chi v5 | Routes, groups, `Mount`, `With`/`Use` middleware, `{param}` paths. |
| gin | Routes on `*gin.Engine` and `*gin.RouterGroup`, groups, `:param` paths normalised to `{param}`. |
| echo v4 | Routes on `*echo.Echo` and `*echo.Group`, groups, `:param` paths normalised to `{param}`. |
| net/http | Go 1.22+ `ServeMux` method patterns: `mux.HandleFunc("GET /users/{id}", h)`. |
| anything else | Add a `routerDefinitions` entry. See the comments in `.respec.yaml` for every field. |

Each of these has a golden-file fixture under `internal/testsuite/testdata`, so
support is asserted rather than claimed.

Routers are traced through variables, function parameters, and return values, so
the usual ways of splitting up route registration all work:

```go
// registration in a helper function
func RegisterUserRoutes(r chi.Router) { ... }
RegisterUserRoutes(r)

// a group held in a variable (the usual gin and echo style)
v1 := r.Group("/v1")
v1.GET("/users", ...)

// a sub-router built elsewhere and mounted
r.Mount("/health", healthRouter())
```

## Known limitations

- **Paths must be literals.** A route whose path is assembled at run time cannot
  be resolved. respec reports each one with a file and line rather than dropping
  it silently.
- **gorilla/mux.** `r.HandleFunc(p, h).Methods("GET")` takes its verb from a
  trailing chained call, which `endpointPatterns` cannot describe yet.
- **Enums and validation constraints.** Schemas carry types, property names,
  `required`, formats, and nullability, but not enum values, ranges, or
  constraints from `validate` tags.
- **`securityPatterns` receiver syntax.** A pointer-receiver method needs a
  leading `*` here (`*pkg.Service.Validate`) but not under `handlerPatterns`.
  This inconsistency is a bug, not a design decision.
- **Interface handlers.** A handler reached only through an interface value
  cannot be traced to a concrete function, so its body is not analyzed. The
  route still appears, with a diagnostic.
- **`ServeMux` patterns without a method.** `mux.HandleFunc("/x", h)` matches
  every verb, but is documented once as `GET` rather than five times. Write
  `"GET /x"` to be explicit. `respec -v` reports each one.

## Contributing

```bash
git checkout -b feature/my-feature
go test ./...
```

Behaviour is pinned by golden-file tests. Each directory under
`internal/testsuite/testdata` is a small, self-contained Go module exercising one
routing style, paired with the exact spec it should produce.

If a change is meant to alter output, regenerate the goldens and **read the diff**
before committing it — that diff is the review:

```bash
go test ./internal/testsuite -update
git diff internal/testsuite/testdata
```

Adding a fixture is the best way to report a routing style respec gets wrong:
create a directory with a `go.mod` (with a `replace` back to the repo root) and a
`main.go`, run the command above, and commit the generated `want.yaml`.

CI additionally enforces that generated specs pass `respec validate`, that the
goldens are current, and that the public `respec` package still has zero
dependencies.

`make help` lists the build, install, lint, and release targets.

## License

MIT — see [LICENSE](./LICENSE).
