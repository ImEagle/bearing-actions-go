# Bearing UML Analyzer for Go

A reusable GitHub Action that analyzes Go code and generates UML structure in JSON format. Optionally uploads the generated JSON to a specified endpoint.

## Features

- Parses Go source code using the standard library AST parser
- Extracts structs, interfaces, functions, methods, and type parameters
- Supports Go generics (type parameters and constraints)
- Outputs structured JSON with module, package, and type information
- Optional upload to external endpoints with authentication

## Usage

### Basic Usage

```yaml
name: Generate UML

on:
  push:
    branches: [main]
  pull_request:

jobs:
  uml:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Generate UML JSON
        uses: your-org/bearing-actions-go@v1
        with:
          path: '.'
          output: 'uml.json'
```

### With Upload

```yaml
name: Generate and Upload UML

on:
  push:
    branches: [main]

jobs:
  uml:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Generate and Upload UML
        uses: your-org/bearing-actions-go@v1
        with:
          path: '.'
          output: 'uml.json'
          project-name: 'my-go-project'
          upload-url: ${{ secrets.DC_UPLOAD_URL }}
          token: ${{ secrets.DC_TOKEN }}
          system-element-id: ${{ secrets.DC_SYSTEM_ELEMENT_ID }}
          dry-run: 'false'
```

### Using Environment Variables

```yaml
name: Generate UML with Env Vars

on:
  push:
    branches: [main]

jobs:
  uml:
    runs-on: ubuntu-latest
    env:
      DC_UPLOAD_URL: ${{ secrets.DC_UPLOAD_URL }}
      DC_TOKEN: ${{ secrets.DC_TOKEN }}
      DC_SYSTEM_ELEMENT_ID: ${{ secrets.DC_SYSTEM_ELEMENT_ID }}
    steps:
      - uses: actions/checkout@v4

      - name: Generate and Upload UML
        uses: your-org/bearing-actions-go@v1
        with:
          dry-run: 'false'
```

## Inputs

| Input | Description | Required | Default |
|-------|-------------|----------|---------|
| `path` | Path to the Go project to analyze | No | `.` |
| `output` | Output filename for the generated JSON | No | `uml.json` |
| `project-name` | Project name to store in JSON metadata | No | - |
| `exclude` | Comma-separated directory names to ignore | No | `.git,.idea,.vscode,node_modules,testdata,vendor` |
| `include-tests` | Include `*_test.go` files in analysis | No | `false` |
| `include-generated` | Include files with "Code generated" headers | No | `false` |
| `pretty` | Pretty-print the JSON output (indent with 2 spaces) | No | `true` |
| `upload-url` | URL to upload the generated file | No | - |
| `token` | Authentication token for upload | No | - |
| `system-element-id` | System element ID for upload | No | - |
| `dry-run` | If true, print the curl command instead of executing | No | `true` |

## Outputs

| Output | Description |
|--------|-------------|
| `output-file` | Path to the generated JSON file |
| `curl-command` | The curl command that would be executed (in dry-run mode) |

## Environment Variables

The following environment variables can be used as fallbacks for inputs:

| Variable | Fallback for |
|----------|--------------|
| `DC_UPLOAD_URL` | `upload-url` |
| `DC_TOKEN` | `token` |
| `DC_SYSTEM_ELEMENT_ID` | `system-element-id` |

## Output JSON Format

The generated JSON follows this structure:

```json
{
  "generated_at": "2024-01-15T10:30:00Z",
  "root": "/path/to/project",
  "module": {
    "path": "github.com/example/project",
    "dir": "/path/to/project"
  },
  "packages": [
    {
      "name": "main",
      "import_path": "github.com/example/project/cmd/app",
      "dir": "cmd/app",
      "files": ["main.go"],
      "types": [
        {
          "name": "Config",
          "kind": "struct",
          "exported": true,
          "doc": "Config holds application configuration",
          "fields": [
            {
              "name": "Host",
              "type": "string",
              "tag": "json:\"host\"",
              "exported": true
            }
          ],
          "methods": [
            {
              "name": "Validate",
              "exported": true,
              "receiver": "*Config",
              "results": [{"type": "error"}]
            }
          ]
        }
      ],
      "functions": [
        {
          "name": "main",
          "exported": false
        }
      ]
    }
  ]
}
```

### Type Kinds

- `struct` - Go struct types
- `interface` - Go interface types
- `alias` - Type aliases (`type Foo = Bar`)
- `other` - Other type definitions

## Local Development

### Build

```bash
go build -o bearing-go ./cmd/umljson
```

### Run

```bash
# Analyze current directory
./bearing-go

# Analyze specific path
./bearing-go /path/to/project

# Write to file
./bearing-go -o uml.json /path/to/project

# Include test files
./bearing-go -tests /path/to/project

# Custom exclude directories
./bearing-go -exclude ".git,vendor,testdata" /path/to/project

# Compact JSON output
./bearing-go -indent "" /path/to/project
```

### Test Locally (Simulating GitHub Actions)

```bash
# Set environment variables
export GITHUB_WORKSPACE=/path/to/your/project
export GITHUB_ACTION_PATH=/path/to/bearing-actions-go
export INPUT_PATH="."
export INPUT_OUTPUT="uml.json"
export INPUT_PRETTY="true"
export INPUT_DRY_RUN="true"

# Build and run
cd $GITHUB_ACTION_PATH
go build -o bearing-go ./cmd/umljson
bash main.sh
```

## Upload Payload Format

When uploading, the action sends a multipart form with the following fields:

| Field | Description |
|-------|-------------|
| `file` | The generated JSON file (`application/json`) |
| `projectName` | Project name (if provided) |
| `systemElementId` | System element ID (if provided) |
| `commitId` | Git commit SHA |
| `branch` | Git branch name |

## License

MIT License
