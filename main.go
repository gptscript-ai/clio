package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/acorn-io/cmd"
	"github.com/adrg/xdg"
	"github.com/fatih/color"
	"github.com/gptscript-ai/clio/internal"
	"github.com/gptscript-ai/go-gptscript"
	"github.com/gptscript-ai/gptscript/pkg/embedded"
	"github.com/gptscript-ai/tui"
	"github.com/spf13/cobra"
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
	LogFile string `usage:"Event log file"`
}

func (c Clio) Customize(cmd *cobra.Command) {
	cmd.Use = "clio [flags] [CUSTOM_AGENT_FILE...]"
	cmd.Short = "Clio - AI powered assistant for your command line."
}

func (c Clio) Run(cmd *cobra.Command, args []string) (err error) {
	if c.APIKey == "" {
		fmt.Println(color.YellowString("Checking authentication..."))
		c.APIKey, c.BaseURL, err = internal.TokenAndURL(cmd.Context(), internal.AppName)
		if err != nil {
			return err
		}
	}

	fmt.Println(color.YellowString("Starting up... (first run takes longer, like a minute, be patient this will get faster next time)"))

	tool, err := getTool(cmd.Context(), c.BaseURL, c.APIKey, args)
	if err != nil {
		return err
	}

	workspace, err := xdg.ConfigFile(filepath.Join(internal.AppName, "workspace"))
	if err != nil {
		return err
	}

	return tui.Run(context.Background(), mainAgent, tui.RunOptions{
		Eval:    []gptscript.ToolDef{tool},
		AppName: internal.AppName,
		TrustedRepoPrefixes: []string{
			"github.com/gptscript-ai",
		},
		OpenAIBaseURL: c.BaseURL,
		OpenAIAPIKey:  c.APIKey,
		Workspace:     workspace,
		Location:      mainAgent,
		EventLog:      c.LogFile,
	})
}

func validateScript(ctx context.Context, c gptscript.GPTScript, path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	nodes, err := c.Parse(ctx, absPath)
	if err != nil {
		return "", err
	}

	for _, node := range nodes {
		if node.ToolNode != nil {
			if !node.ToolNode.Tool.Chat || !slices.Contains(node.ToolNode.Tool.Context, internal.Context) {
				return "", fmt.Errorf("invalid agent file: %s, agents must include 'chat: true' and 'context: %s'",
					path, internal.Context)
			}
			return absPath, nil
		}
	}

	return "", fmt.Errorf("invalid agent file: %s, agents must include 'chat: true' and 'context: %s' in the first "+
		"tool definition", path, internal.Context)
}

func agentsFromHomeConfig(ctx context.Context, c gptscript.GPTScript) (result []string, _ error) {
	dir, err := xdg.ConfigFile(internal.AppName + "/agents")
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	for _, f := range files {
		path, err := validateScript(ctx, c, filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, err
		}

		result = append(result, path)
	}

	return
}

func getTool(ctx context.Context, url, key string, args []string) (tool gptscript.ToolDef, _ error) {
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

	if tool.Name == "" {
		return tool, errors.New("failed to find tool " + mainAgent)
	}

	toolsFromConfig, err := agentsFromHomeConfig(ctx, c)
	if err != nil {
		return tool, err
	}

	tool.Agents = append(tool.Agents, toolsFromConfig...)

	if len(args) > 0 {
		var newArgs []string
		for _, arg := range args {
			newArg, err := validateScript(ctx, c, arg)
			if err != nil {
				return tool, err
			}
			newArgs = append(newArgs, newArg)
		}

		return gptscript.ToolDef{
			Agents: append(newArgs, tool.Agents...),
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
