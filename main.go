package main

import (
	"context"
	"embed"
	"github.com/acorn-io/cmd"
	"github.com/adrg/xdg"
	"github.com/gptscript-ai/go-gptscript"
	"github.com/gptscript-ai/gptscript/pkg/embedded"
	"github.com/gptscript-ai/clio/internal"
	"github.com/gptscript-ai/tui"
	"github.com/spf13/cobra"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed agent.gpt context/*.gpt tools/*.gpt agents/*.gpt
var embedFS embed.FS

const (
	mainAgent = "/internal/agent.gpt"
)

type internalFS struct{}

func (internalFS) Open(name string) (fs.File, error) {
	if name, ok := strings.CutPrefix(name, "/internal/"); ok {
		return embedFS.Open(name)
	}
	return os.Open(name)
}

type Clio struct {
	BaseURL string `usage:"OpenAI base URL" name:"openai-base-url" env:"OPENAI_BASE_URL"`
	APIKey  string `usage:"OpenAI API KEY" name:"openai-api-key" env:"OPENAI_API_KEY"`
}

func (c Clio) Run(cmd *cobra.Command, args []string) (err error) {
	if c.APIKey == "" {
		c.APIKey, c.BaseURL, err = internal.TokenAndURL(cmd.Context(), internal.AppName)
		if err != nil {
			return err
		}
	}

	tool, err := getTool(c.BaseURL, c.APIKey)
	if err != nil {
		return err
	}

	workspace, err := xdg.ConfigFile(filepath.Join(internal.AppName, "workspace"))
	if err != nil {
		return err
	}

	return tui.Run(context.Background(), internal.AppName, tui.RunOptions{
		Eval:    []gptscript.ToolDef{tool},
		AppName: internal.AppName,
		TrustedRepoPrefixes: []string{
			"github.com/gptscript-ai",
		},
		OpenAIBaseURL: c.BaseURL,
		OpenAIAPIKey:  c.APIKey,
		Workspace:     workspace,
	})
}

func getTool(url, key string) (tool gptscript.ToolDef, _ error) {
	c, err := gptscript.NewGPTScript(gptscript.GlobalOptions{
		OpenAIBaseURL: url,
		OpenAIAPIKey:  key,
	})
	if err != nil {
		return tool, err
	}
	// purposely not closing client otherwise it does a start/stop thing and I don't like that

	nodes, err := c.Parse(context.Background(), mainAgent)
	if err != nil {
		return tool, err
	}

	for _, node := range nodes {
		if node.ToolNode != nil {
			tool = node.ToolNode.Tool.ToolDef
		}
	}

	addons := os.Args[1:]
	if len(addons) > 0 {
		return gptscript.ToolDef{
			Agents: append(addons, tool.Agents...),
		}, nil
	}

	return tool, nil
}

func main() {
	if embedded.Run(embedded.Options{FS: internalFS{}}) {
		return
	}
	cmd.Main(cmd.Command(&Clio{}))
}
