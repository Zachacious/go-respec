<h1 align="center">📘 respec</h1>
<p align="center"><em>Forget magic comments. If your Go source code is your source of truth, then <strong>respec</strong> is the static analysis tool you've been waiting for.</em></p>

<p align="center">
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

| Method               | Description                                      | Example (Handler)                | Example (Meta)                   |
| -------------------- | ------------------------------------------------ | -------------------------------- | -------------------------------- |
| .Summary(string)     | Short name of the operation                      | .Summary("Get User by ID")       | .Summary("User Administration")  |
| .Description(string) | Longer operation description                     | .Description("Retrieves a user") | .Description("Manages users...") |
| .Tag(...string)      | One or more OpenAPI tags                         | .Tag("Users", "Profiles")        | .Tag("Admin")                    |
| .Security(string)    | Reference to a security scheme in `.respec.yaml` | .Security("BearerAuth")          | .Security("AdminSecurity")       |

---

### 📎 Examples

Summary:

```go
r.Get("/{id}",
  respec.Handler(handler).
    Summary("Get User by ID").
    Unwrap(),
)
```

Description:

```go
r.Get("/{id}",
  respec.Handler(handler).
    Description("Retrieves the user profile.").
    Unwrap(),
)
```

Tags:

```go
r.Get("/{id}", respec.Handler(handler).Tag("User Management").Unwrap())

// Multiple tags
r.Get("/{id}/sessions",
  respec.Handler(handler).
    Tag("User Management", "Sessions").
    Unwrap(),
)
```

Security:

```go
r.Get("/me",
  respec.Handler(handler).
    Security("BearerAuth").
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
