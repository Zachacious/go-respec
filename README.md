<h1 align="center">Go Respec</h1>
<p align="start"><em>Forget magic comments. If your Go source code is your source of truth, then <strong>respec</strong> is the static analysis tool you've been waiting for.</em></p>

<p align="start">
  <strong>Generate OpenAPI v3 specs from Go code — cleanly, powerfully, and without polluting your codebase with annotations.</strong>
</p>

---

![Go Respec](images/respec2.png)

---

## 💡 What is respec?

respec is a powerful, framework-agnostic CLI tool that introspects your Go source code to generate a production-grade OpenAPI specification.

It is built on a philosophy of:

- smart inference
- sensible defaults
- explicit—but unobtrusive—overrides

---

## 🚨 Disclaimer

This is a new and experimental project built primarily for my own needs after being frustrated with existing tools that rely heavily on "magic comments."

It is beta-quality but robust, having been tested against a non-trivial chi project. Feedback and contributions are welcome!

---

## 🧠 The respec Philosophy

Traditional OpenAPI generators require you to annotate your code to death. Not respec.

respec uses a 3-layered design:

### 1️⃣ Layer 1: Explicit Metadata API (Highest Priority)

Override inferred values in your Go code using a clean, fluent API — route by route.

### 2️⃣ Layer 2: Doc Comments (Fallback)

respec parses doc comments when metadata is missing.

### 3️⃣ Layer 3: Smart Inference (Default)

The static analysis engine infers:

- Routing structure and middleware
- Operation summaries from functions
- Query/path/header parameters
- Request/response bodies
- Multiple response codes and schemas

You get a full working spec with minimal effort — and the tools to perfect it.

---

## ✨ Features

- 🧼 Zero Magic Comments — no annotations or clutter
- 🧠 Powerful Static Analysis — routes, parameters, schemas, and more auto-detected
- 🔌 Framework Agnostic — works with chi, gin, echo, and more
- ⚙️ Configurable Inference — fully customizable via .respec.yaml
- 🧱 Three-Layered Inference — convention over configuration, with overrides when needed
- 🧩 Middleware-Aware — detects security and route scopes from applied middleware

---

## 🚀 Installation

### Via go install (Recommended)

```bash
go install github.com/Zachacious/go-respec/cmd/respec@latest
```

### From Release Binaries

Download the appropriate binary from the [Releases](https://github.com/Zachacious/go-respec/releases) page for your OS.

---

## ⚡ Quick Start

1. From your project root:

```bash
cd /path/to/your/project
```

2. Initialize respec: (global settings)

```
respec init
```

3. Generate a spec:

```bash
respec . -o openapi.yaml
```

3. Inspect the generated openapi.yaml — you’ll find a surprisingly complete spec.

---

## 📦 Metadata API Reference (Layer 1)

respec allows you to decorate your handlers for full control over the spec.

### ✅ Import the Library

```go
import "github.com/Zachacious/go-respec/respec"
```

---

### 📍 respec.Handler() for Individual Routes

Upgrade your route handlers like this:

Before:

```go
r.With(mw.Authenticator).Post("/users", userHandlers.Create)
```

After:

```go
r.With(mw.Authenticator).Post("/users",
  respec.Handler(userHandlers.Create).
    Tag("User Management").
    Summary("Create a new system user").
    Security("BearerAuth").
    Unwrap(),
)
```

> Note: .Unwrap() is required to return the original handler func.

---

### 📍 respec.Meta() for Group Routes

Wrap a routing group to apply metadata to all routes inside:

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

---

### 🔄 Available Methods

| Method                                    | Description                                                                                             | Applies To    |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------- | ------------- |
| `.Summary(string)`                        | Overrides the summary (a short title) for the operation(s).                                             | Handler       |
| `.Description(string)`                    | Overrides the longer description for the operation(s).                                                  | Handler       |
| `.Tag(...string)`                         | Sets tags for the operation(s). Replaces any inherited tags.                                            | Handler, Meta |
| `.Security(...string)`                    | Sets security schemes. Replaces any inherited security.                                                 | Handler, Meta |
| `.RequestBody(obj)`                       | Overrides the request body with a schema generated from `obj`.                                          | Handler only  |
| `.AddResponse(code, content)`             | Adds or overrides a response. `content` can be a struct (`User{}`) or a string literal (`"Not Found"`). | Handler only  |
| `.AddParameter(in, name, desc, req, dep)` | Adds or overrides a parameter (`in` is `"query"`, `"header"`, etc.).                                    | Handler only  |
| `.ResponseHeader(code, name, desc)`       | Adds a header to a specific response code.                                                              | Handler only  |
| `.OperationID(string)`                    | Sets a custom `operationId` for the endpoint.                                                           | Handler only  |
| `.Deprecate(bool)`                        | Marks the operation(s) as deprecated.                                                                   | Handler, Meta |
| `.ExternalDocs(url, desc)`                | Adds a link to external documentation for the operation.                                                | Handler only  |
| `.AddServer(url, desc)`                   | Adds an operation-specific server URL.                                                                  | Handler only  |

---

### 📎 Examples

This example demonstrates how to use the full suite of override methods to take complete control of an endpoint's specification.

```go
// Define custom structs for request and response schemas.
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

// In your router setup:
r.Post("/users",
    respec.Handler(userHandlers.Create).
        Summary("Create a new system user").
        Description("Creates a new user account with the provided information. Returns the user object on success.").
        Tag("Users", "Write Ops").
        Security("BearerAuth", "ApiKeyAuth").
        OperationID("users-create").
        Deprecate(false).

        // Override request and response bodies
        RequestBody(CreateUserRequest{}).
        AddResponse(201, UserResponse{}).
        AddResponse(409, ErrorResponse{}).
        AddResponse(400, "Invalid request payload provided.").

        // Add custom parameters and response headers
        AddParameter("query", "source", "The source of the registration.", false, false).
        ResponseHeader(201, "X-RateLimit-Remaining", "The number of requests remaining for this client.").

        // Add external documentation and an operation-specific server
        ExternalDocs("[https://docs.example.com/users/create](https://docs.example.com/users/create)", "API Usage Guide").
        AddServer("[https://users.api.example.com](https://users.api.example.com)", "Users API Server").

        Unwrap(),
)
```

---

## 🖥️ CLI Usage

### 🌐 Generate Spec (default)

```bash
respec [path] [flags]
```

- [path] (optional): directory of your Go project (default: current directory)
- Flags:
  - -o, --output <file>: Set output path (use file extension to select YAML or JSON)
  - -h, --help: Show help

Examples:

```bash
respec                   # -> generates openapi.yaml
respec ./project -o api.json
```

---

### 🛠️ Other Commands

| Command         | Description                                   |
| --------------- | --------------------------------------------- |
| respec init     | Create default .respec.yaml (non-destructive) |
| respec version  | Print the current version of the tool         |
| respec validate | Validate a YAML/JSON spec against OpenAPI 3.1 |

Example validate usage:

```bash
respec validate openapi.yaml
respec validate specs/api.json
```

---

## 📖 .respec.yaml Configuration

This file is the control panel for customizing inference. You can generate one with:

```bash
respec init
```

> See the source repo for a full example or start with the one generated by `init`.

---

## 🤝 Contributing

This tool was developed for real-world needs, and I welcome contributions!

### How to contribute:

```bash
# 1. Fork the repo
# 2. Create a feature branch
git checkout -b feature/AmazingFeature

# 3. Commit and push
git commit -m "Add feature"
git push origin feature/AmazingFeature

# 4. Open a pull request
```

---

## 📜 License

MIT License — see the [LICENSE](./LICENSE) file for full terms.
