# Clio - AI Powered Assistant for Your Command Line

Clio is an AI-powered assistant designed to help you with DevOps-related tasks using CLI programs. It leverages OpenAI's capabilities to provide intelligent assistance directly from your command line.

## Features

- **Kubernetes Management**: Interact with your Kubernetes clusters using `kubectl`, `helm`, and other CLIs.
- **Cloud Integration**: Manage your AWS, GCP, and DigitalOcean resources.
- **GitHub Integration**: Interact with your GitHub repositories.
- **Easy Customizable**: Add new capabilities with no code.

## Installation

To install Clio, you can use Homebrew:

```bash
brew install gptscript-ai/tap/clio
```

Alternatively, you can clone the repository and build the project manually:

```bash
git clone https://github.com/gptscript-ai/clio.git
cd clio
make build
```

## Usage

To start Clio, simply run:

```bash
clio
```

Clio will prompt you to authenticate with GitHub if you haven't provided an OpenAI API key. You can also set a custom personal API key and base URL using environment variables:

```bash
export OPENAI_API_KEY=your-api-key
export OPENAI_BASE_URL=https://api.openai.com/v1
```

## Extending

### Agents

Clio internally is a collection of agents. There a several built in agents that provide functionality for interacting with Kubernetes, AWS, GCP, GitHub, etc, but you can easily add your own agents to extend the functionality of Clio. The built-in agents are located in the [agents](./agents) directory, with each file being a separate agent. To create a custom agent you must write a new [GPTScript](https://docs.gptscript.ai) and place it in the `$XDG_CONFIG_HOME/clio/agents` directory.`

| Operating System | Custom Agent Path |
|------------------|-----------------------------------|
| macOS            | `~/Library/Application Support/clio/agents` |
| Linux            | `~/.config/clio/agents`           |

### Custom Agent

A custom agent is any GPTScript with the requirement that it minimally must contain the following lines

```gptscript
chat: true
context: github.com/gptscript-ai/clio/context
```
You can refer to the [GPTScript documentation](https://docs.gptscript.ai) for all the capabilites of GPTScripts, but for now the below example is typically
all you need to know.

#### Example Custom Agent - GoReleaser

For this example we will add a custom agent that is specialized for GoReleaser 2. We will create a file called `goreleaser.gpt`. The finished example is available in the [examples](./examples/goreleaser.gpt) directory.

The GPTScript starts with a metadata block that defines the name of the agent, a description, and the required context and chat fields. It is then followed by the prompt that will tell Clio how to behave when this agent is invoked.

```gptscript
Name: GoReleaser
Description: Assistant for GoReleaser 2 using the goreleaser CLI
Chat: true
Context: github.com/gptscript-ai/clio/context

You are an expert at goreleaser. You can run the goreleaser CLI and help manage the goreleaser config file.

Rules:
1. Before changing the config, always show goreleaser config to the user for confirmation. After they agree, then write to disk.
2. Make sure "version: 2" line is always in the goreleaser config.
3. If the user asks to build, do a snapshot build.
4. Always search the internet for relavent information when asked a question or to do a task.

First as the user what would they like to do with regards to GoReleaser.
```

To test this agent out you can run `clio goreleaser.gpt`. After testing the agent you can modify the text if you don't like the exhibited behavior. There
is not defined format for the prompt. The fact that the example has the structure with "Rules" in it is just a convention but not technically required.

To make the agent even more useful you can extend the agent to have dynamic contextual information. To do this we are going to add an additional "context tool" to the agent. Context tools add capabilites to agent by prepending the output of the tool to the prompt.

In the below example we add a new line for `context: additional-environment` to the metadata block. We then define the `additional-environment` context tool. This tool will show the user the current goreleaser version, the JSONSchema for the goreleaser config file, and the help output for the goreleaser CLI and the build subcommand. It will also show the user the current goreleaser config file if it exists.

```gptscript
Name: GoReleaser
Description: Assistant for GoReleaser 2 using the goreleaser CLI
Chat: true
Context: github.com/gptscript-ai/clio/context
Context: additional-environment

You are an expert at goreleaser. You can run the goreleaser CLI and help manage the goreleaser config file.

Rules:
1. Before changing the config, always show goreleaser config to the user for confirmation. After they agree, then write to disk.
2. Make sure "version: 2" line is always in the goreleaser config.
3. If the user asks to build, do a snapshot build.
4. Always search the internet for relavent information when asked a question or to do a task.

First as the user what would they like to do with regards to GoReleaser.

---
Name: additional-environment
Tools: sys.exec

#!/bin/bash

goreleaser --version || true

echo 'The JSONSchema for .goreleaser.yaml is as follows:'
goreleaser jsonschema || true


echo Additional CLI help
echo

goreleaser --help || true
goreleaser build --help || true

if [ -e .goreleaser.yaml ]; then
  echo
  echo "The current .goreleaser.yaml:"
  echo
  echo '```yaml'
  cat .goreleaser.yaml
  echo '```'
fi

```

That is now our finished agent. You can place the `goreleaser.gpt` file in the `$XDG_CONFIG_HOME/clio/agents` directory and then the next time you run `clio` you will see your agent in the list of agents and it can be referenced by doing `@gorelease <your question>`.

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.