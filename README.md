![Go Reference](images/respec.jpg)

# respec

> A Go static analysis tool for generating OpenAPI v3 specifications with zero magic comments.

**respec** is a powerful, framework-agnostic CLI tool that introspects your Go source code to generate a production-grade OpenAPI specification. It is built on a philosophy of smart inference, sensible defaults, and explicit-but-unobtrusive overrides.

---

## 🚨 Disclaimer

This is a new and experimental project that I created for my own purposes after being unhappy with existing solutions that rely heavily on "magic comments" polluting the source code.

It is designed to be robust and has been tested on a non-trivial chi project, but I consider it to be in a beta stage. I am actively looking for feedback and am happy to help anyone make this work for their project.

---

## 🧠 The respec Philosophy

Traditional OpenAPI generators for Go often require you to litter your code with special annotations or "magic comments". This couples your documentation tightly to your implementation and clutters your source code.

**respec** is different. It uses a 3-layer approach to generating a specification, prioritizing convention and inference over explicit annotation.

### Layer 1: Explicit Metadata API (Highest Priority)

For ultimate control, `respec` provides a clean, fluent Go API to wrap your route definitions. This allows you to explicitly override any inferred value, from a description to a security scheme. This is your "escape hatch" for perfecting the spec.

### Layer 2: Doc Comments (Fallback)

If no explicit metadata is provided, `respec` will intelligently parse the standard Go doc comments above your handler functions to use as the summary and description for an operation.

### Layer 3: Smart Inference (The Foundation)

This is where the magic happens. The powerful static analysis engine at the core of `respec` reads your source code to infer almost everything:

- The routing tree, including groups and middleware chains.
- Path, query, and header parameters.
- Request body objects.
- Multiple response codes and their corresponding schemas.

This layered approach means you get a high-quality spec out-of-the-box with minimal effort, while still having powerful tools to refine it when needed.

---

## ✨ Features

- 🧼 **Zero Magic Comments**: Keep your source code clean and free of special annotations.
- 🧠 **Powerful Static Analysis**: Intelligently infers routes, request bodies, responses, parameters, and more.
- 🔌 **Framework-Agnostic**: Works with any Go web framework (`chi`, `gin`, `echo`, etc.) through a simple configuration file.
- ⚙️ **Configurable Inference**: A comprehensive `.respec.yaml` file allows you to teach `respec` about your project's custom helper functions.
- 🧱 **Three-Layered System**: Intelligent hierarchy of inference, doc comments, and explicit Go API.
- 🧩 **Middleware-Aware**: Can infer properties like security schemes by analyzing the middleware applied to your routes.

---

## 🚀 Installation

You can install respec in two ways:

### Via `go install` (Recommended)

```bash
go install github.com/Zachacious/go-respec/cmd/respec@latest
```

### From Release Binaries

Alternatively, you can download the pre-compiled binary for your operating system from the [GitHub Releases page](https://github.com/Zachacious/go-respec/releases).

---

## ⚡ Quick Start

1. Navigate to your project's root directory:

```bash
cd /path/to/your/project
```

2. Generate your specification:

```bash
respec
```

3. (Optional) Add overrides using the `respec.Route` API:

```go
import "github.com/Zachacious/go-respec/respec"

...

respec.Route(r.Post("/users", userHandlers.Create)).
    Tag("User Management").
    Summary("Create a new system user")
```

---

## 🖥️ CLI Usage

The `respec` command-line interface is simple and straightforward.

### Generate Command (Default)

This is the main command for generating the OpenAPI spec.

**Usage:**

````bash
respec [path] [flags]

#### Arguments

- `[path]` (optional): The path to the root of your Go project. Defaults to current dir (`.`).

#### Flags

- `-o`, `--output <file>`: Specifies the output file for the spec. The format (YAML or JSON) is automatically determined by the file extension. (default: `openapi.yaml`)
- `-h`, `--help`: Displays the help message

#### Examples

```bash
# Default
respec

# Specific path and output
respec /path/to/project -o ./specs/api.json
````

---

## 🔧 Other Commands

### `init`

Creates a default `.respec.yaml` file in the current directory. This is the best way to get started with configuration. It will not overwrite an existing file.

```bash
respec init
```

### `version`

Prints the current version of the tool.

```bash
respec version
```

---

## 📖 Configuration (`.respec.yaml`)

This file is the control panel for the inference engine.

### Example:

```yaml
# .respec.yaml - The Complete Guide

# ---------------------------------------------------------------------------
# SECTION 1: API Metadata (Recommended)
# ---------------------------------------------------------------------------
# Purpose: Defines the high-level information for your OpenAPI specification.
# When to use: Always. This gives your generated spec a professional title,
# version, and description.
# Optional: Yes, but highly recommended. `respec` provides a generic
# default if this section is omitted.
info:
  title: "My API"
  version: "1.0.0"
  description: "The complete REST API for the my service."

# ---------------------------------------------------------------------------
# SECTION 2: Security Schemes (Optional)
# ---------------------------------------------------------------------------
# Purpose: Defines the security mechanisms your API uses (e.g., JWT, API Keys).
# When to use: Use this section to give a name and definition to each security
# type you use. This definition will be referenced later by securityPatterns.
# Optional: Yes. Only needed if your API has secured endpoints.
securitySchemes:
  # 'BearerAuth' is a custom name you choose. You will use this name later.
  BearerAuth:
    type: http # The type of security. 'http' is common for tokens.
    scheme: bearer # The scheme, e.g., 'bearer' for JWTs.
    bearerFormat: JWT # A hint about the format.

# ---------------------------------------------------------------------------
# SECTION 3: Router Definitions (Optional, for non-standard frameworks)
# ---------------------------------------------------------------------------
# Purpose: Teaches `respec` the routing syntax of your web framework.
# When to use: Only if you are using a framework other than Chi or Gin.
# Optional: Yes. `respec` has built-in defaults for `chi/v5` and `gin-gonic/gin`.
# You do NOT need to include this section if you use one of those frameworks.
# It is shown here for educational purposes.
routerDefinitions:
  - # This is the built-in definition for the Chi router.
    type: "github.com/go-chi/chi/v5.Mux"
    endpointMethods:
      ["Get", "Post", "Put", "Patch", "Delete", "Head", "Options", "Trace"]
    groupMethods: ["Route", "Group"]
    middlewareWrapperMethods: ["With", "Use"]

# ---------------------------------------------------------------------------
# SECTION 4: Handler Inference Patterns (Optional, for custom helpers)
# ---------------------------------------------------------------------------
# Purpose: This is the most powerful section. It teaches `respec` to infer
# details by recognizing your project's custom helper functions.
# When to use: When your handlers don't use the standard library directly, but
# instead use custom utility functions to write responses or bind requests.
# Optional: Yes. `respec` has built-in magic for standard library functions.
# You only need to add patterns for your project's specific helpers.
handlerPatterns:
  # Defines functions that parse the request body.
  requestBody:
    # This teaches respec that `utils.ValidateRequest(&req, ...)` means the
    # first argument (`argIndex: 0`) is the request body struct.
    - functionPath: "github.com/me/myservice/internal/utils.ValidateRequest"
      argIndex: 0

  # Defines functions that write HTTP responses.
  responseBody:
    # This pattern matches your `utils.RespondWithJSON` helper.
    - functionPath: "github.com/me/myservice/internal/utils.RespondWithJSON"
      statusCodeIndex: 1 # The 2nd argument (index 1) is the status code.
      dataIndex: 2 # The 3rd argument (index 2) is the response data.

    # This pattern matches your `utils.RespondWithError` helper.
    - functionPath: "github.com/me/myservice/internal/utils.RespondWithError"
      statusCodeIndex: 1 # The 2nd argument is the status code.
      descriptionIndex: 2 # The 3rd argument is the error message string.
      dataIndex: 3 # The 4th argument is the error data object.

  # Defines functions for reading query parameters.
  # Optional: The standard library default is built-in, shown here for example.
  queryParameter:
    - functionPath: "net/http.URL.Query.Get"
      nameIndex: 0

  # Defines functions for reading header parameters.
  # Optional: The standard library default is built-in, shown here for example.
  headerParameter:
    - functionPath: "net/http.Header.Get"
      nameIndex: 0

# ---------------------------------------------------------------------------
# SECTION 5: Security Inference Patterns (Optional)
# ---------------------------------------------------------------------------
# Purpose: Connects a function call found in your middleware to a security
# scheme you defined in `securitySchemes`.
# When to use: When you want `respec` to automatically document which endpoints
# are protected.
# Optional: Yes. Use this to enable security inference.
securityPatterns:
  # This rule tells respec: "When you see a call to the 'Validate' method
  # on a 'token.Service' anywhere inside a middleware, apply the 'BearerAuth'
  # security scheme to all routes protected by that middleware."
  - functionPath: "github.com/me/myservice/internal/services/token.Service.Validate"
    schemeName: "BearerAuth"
```

---

## 🤝 Contributing

This project was born out of a personal need and has primarily been tested with a single non-trivial `chi`-based project.

Bug fixes, ideas, and feedback are welcome! 💬

### How to contribute:

1. Fork the repository
2. Create a new branch
   ```bash
   git checkout -b feature/AmazingFeature
   ```
3. Commit your changes
   ```bash
   git commit -m 'Add some AmazingFeature'
   ```
4. Push to GitHub
   ```bash
   git push origin feature/AmazingFeature
   ```
5. Open a Pull Request

---

## 📜 License

This project is licensed under the MIT License — see the [LICENSE](./LICENSE) file for details.
