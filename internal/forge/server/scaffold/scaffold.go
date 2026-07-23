package scaffold

import (
	"fmt"

	"github.com/ceasarb/demigo-tools/internal/forge/shared/templates"
)

// Language is the server programming language.
type Language string

const (
	Python     Language = "python"
	TypeScript Language = "typescript"
)

// Transport is the MCP transport protocol.
type Transport string

const (
	Stdio Transport = "stdio"
	HTTP  Transport = "http"
)

// Options for scaffolding a new MCP server.
type Options struct {
	Name        string
	Language    Language
	Transport   Transport
	Description string
	Author      string
	OutputDir   string
}

// Run generates the server project files.
func Run(opts Options) ([]templates.File, error) {
	ctx := templates.Context{
		ServiceName:   opts.Name,
		ToolPrefix:    toSnake(opts.Name),
		PythonPackage: toSnake(opts.Name),
		Description:   opts.Description,
		Author:        opts.Author,
		Transport:     string(opts.Transport),
	}

	key := string(opts.Language) + "-" + string(opts.Transport)
	tmplSet, ok := templateSets[key]
	if !ok {
		return nil, fmt.Errorf("unsupported template: %s", key)
	}

	var files []templates.File
	for _, ft := range tmplSet {
		content, err := templates.Render(ft.Template, ctx)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", ft.Path, err)
		}

		path, err := templates.Render(ft.Path, ctx)
		if err != nil {
			return nil, fmt.Errorf("render path %s: %w", ft.Path, err)
		}

		files = append(files, templates.File{
			Path:    path,
			Content: content,
		})
	}

	return files, nil
}

func toSnake(s string) string {
	result := ""
	for i, r := range s {
		if r == '-' || r == ' ' {
			result += "_"
		} else if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result += "_"
			}
			result += string(r + 32)
		} else {
			result += string(r)
		}
	}
	return result
}
