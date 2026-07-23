package scaffold

// fileTemplate pairs a path template with a content template.
type fileTemplate struct {
	Path     string
	Template string
}

var templateSets = map[string][]fileTemplate{
	"python-stdio":      pythonStdioTemplates,
	"python-http":       pythonHTTPTemplates,
	"typescript-stdio":  typescriptStdioTemplates,
}

// --- Python Stdio Templates ---

var pythonStdioTemplates = []fileTemplate{
	{
		Path: "pyproject.toml",
		Template: `[project]
name = "{{.ServiceName}}"
version = "0.1.0"
description = "{{.Description}}"
requires-python = ">=3.11"
dependencies = [
    "fastmcp>=2.0",
]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.hatch.build.targets.wheel]
packages = ["src/{{.PythonPackage}}"]

[project.scripts]
"{{.ServiceName}}" = "{{.PythonPackage}}.server:main"

[tool.demi]
server_module = "{{.PythonPackage}}.server"
command = "uv run {{.ServiceName}}"
fixtures = "tests/fixtures"
`,
	},
	{
		Path: "src/{{.PythonPackage}}/__init__.py",
		Template: "",
	},
	{
		Path: "src/{{.PythonPackage}}/server.py",
		Template: `"""{{.Description}}"""

from fastmcp import FastMCP

mcp = FastMCP("{{.ServiceName}}")


@mcp.tool(
    annotations={
        "readOnlyHint": True,
        "idempotentHint": True,
        "openWorldHint": False,
    }
)
def hello(name: str) -> str:
    """Say hello to someone.

    Args:
        name: The name to greet.
    """
    return f"Hello, {name}! Welcome to {{.ServiceName}}."


def main():
    mcp.run(transport="stdio", show_banner=False)


if __name__ == "__main__":
    main()
`,
	},
	{
		Path: "tests/__init__.py",
		Template: "",
	},
	{
		Path: "tests/fixtures/test_tools.yaml",
		Template: `tests:
  - name: hello tool returns greeting
    tool: hello
    input:
      name: "World"
    expect:
      status: success
      content_contains: "Hello, World!"

  - name: hello tool handles different names
    tool: hello
    input:
      name: "Demigo"
    expect:
      status: success
      content_contains: "Hello, Demigo!"
`,
	},
	{
		Path: "README.md",
		Template: `# {{.ServiceName}}

{{.Description}}

## Getting Started

` + "```bash" + `
# Install dependencies
uv sync

# Run the server
uv run {{.ServiceName}}

# Test with Demigo Forge
demi forge server test
demi forge server dev
` + "```" + `
`,
	},
	{
		Path: "demi.toml",
		Template: `[server]
name = "{{.ServiceName}}"
entry = "src/{{.PythonPackage}}/server.py"
command = "uv run {{.ServiceName}}"
transport = "stdio"

[testing]
fixtures = "tests/fixtures"
`,
	},
}

// --- Python HTTP Templates ---

var pythonHTTPTemplates = []fileTemplate{
	{
		Path: "pyproject.toml",
		Template: `[project]
name = "{{.ServiceName}}"
version = "0.1.0"
description = "{{.Description}}"
requires-python = ">=3.11"
dependencies = [
    "fastmcp>=2.0",
]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.hatch.build.targets.wheel]
packages = ["src/{{.PythonPackage}}"]

[project.scripts]
"{{.ServiceName}}" = "{{.PythonPackage}}.server:main"

[tool.demi]
server_module = "{{.PythonPackage}}.server"
command = "uv run {{.ServiceName}}"
transport = "http"
port = 8000
fixtures = "tests/fixtures"
`,
	},
	{
		Path: "src/{{.PythonPackage}}/__init__.py",
		Template: "",
	},
	{
		Path: "src/{{.PythonPackage}}/server.py",
		Template: `"""{{.Description}}"""

from fastmcp import FastMCP

mcp = FastMCP("{{.ServiceName}}")


@mcp.tool(
    annotations={
        "readOnlyHint": True,
        "idempotentHint": True,
        "openWorldHint": False,
    }
)
def hello(name: str) -> str:
    """Say hello to someone.

    Args:
        name: The name to greet.
    """
    return f"Hello, {name}! Welcome to {{.ServiceName}}."


def main():
    mcp.run(transport="streamable-http", show_banner=False)


if __name__ == "__main__":
    main()
`,
	},
	{
		Path: "tests/__init__.py",
		Template: "",
	},
	{
		Path: "tests/fixtures/test_tools.yaml",
		Template: `tests:
  - name: hello tool returns greeting
    tool: hello
    input:
      name: "World"
    expect:
      status: success
      content_contains: "Hello, World!"
`,
	},
	{
		Path: "README.md",
		Template: `# {{.ServiceName}}

{{.Description}}

## Getting Started

` + "```bash" + `
# Install dependencies
uv sync

# Run the server (HTTP on port 8000)
uv run {{.ServiceName}}

# Test with Demigo Forge
demi forge server test
demi forge server dev
` + "```" + `
`,
	},
	{
		Path: "demi.toml",
		Template: `[server]
name = "{{.ServiceName}}"
entry = "src/{{.PythonPackage}}/server.py"
command = "uv run {{.ServiceName}}"
transport = "http"
port = 8000

[testing]
fixtures = "tests/fixtures"
`,
	},
}

// --- TypeScript Stdio Templates ---

var typescriptStdioTemplates = []fileTemplate{
	{
		Path: "package.json",
		Template: `{
  "name": "{{.ServiceName}}",
  "version": "0.1.0",
  "description": "{{.Description}}",
  "type": "module",
  "scripts": {
    "build": "tsc",
    "start": "node dist/server.js"
  },
  "dependencies": {
    "@modelcontextprotocol/sdk": "^1.0.0",
    "zod": "^3.22.0"
  },
  "devDependencies": {
    "typescript": "^5.3.0",
    "@types/node": "^20.0.0"
  }
}
`,
	},
	{
		Path: "tsconfig.json",
		Template: `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "Node16",
    "moduleResolution": "Node16",
    "outDir": "dist",
    "rootDir": "src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "declaration": true
  },
  "include": ["src/**/*"]
}
`,
	},
	{
		Path: "src/server.ts",
		Template: `import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";

const server = new McpServer({
  name: "{{.ServiceName}}",
  version: "0.1.0",
});

server.tool(
  "hello",
  "Say hello to someone",
  {
    name: z.string().describe("The name to greet"),
  },
  async ({ name }) => ({
    content: [
      {
        type: "text",
        text: ` + "`" + `Hello, ${name}! Welcome to {{.ServiceName}}.` + "`" + `,
      },
    ],
  })
);

async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
}

main().catch(console.error);
`,
	},
	{
		Path: "tests/fixtures/test_tools.yaml",
		Template: `tests:
  - name: hello tool returns greeting
    tool: hello
    input:
      name: "World"
    expect:
      status: success
      content_contains: "Hello, World!"
`,
	},
	{
		Path: "README.md",
		Template: `# {{.ServiceName}}

{{.Description}}

## Getting Started

` + "```bash" + `
# Install dependencies
npm install

# Build
npm run build

# Run the server
npm start

# Test with Demigo Forge
demi forge server test
demi forge server dev
` + "```" + `
`,
	},
	{
		Path: "demi.toml",
		Template: `[server]
name = "{{.ServiceName}}"
entry = "src/server.ts"
command = "node dist/server.js"
transport = "stdio"

[testing]
fixtures = "tests/fixtures"
`,
	},
}
