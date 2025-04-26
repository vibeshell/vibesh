# Vibesh

Vibesh is an AI-enhanced interactive shell that allows you to execute commands in different modes:

1. **Direct mode** - Commands are executed directly in the underlying shell
2. **AI mode** - Natural language is converted to shell commands using AI
3. **RAG mode** - Commands are matched against a knowledge base of common commands

## Requirements

- Go 1.16+ installed
- OpenAI API key (for AI and RAG modes with full functionality)

## Installation

Clone this repository and build:

```bash
git clone https://github.com/yourusername/vibesh.git
cd vibesh
go build
```

## Usage

### Setting up your API key

For AI and RAG modes to work with full functionality, set your OpenAI API key:

```bash
export OPENAI_API_KEY=your_openai_api_key_here
```

### Running Vibesh

```bash
./vibesh
```

### Using with DevContainer

Vibesh can be run inside a DevContainer for an isolated development environment:

1. Install [Docker](https://www.docker.com/get-started/) and [VS Code](https://code.visualstudio.com/)
2. Install the [Remote - Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension in VS Code
3. Copy `devcontainer.env.template` to `.env.devcontainer` and add your OpenAI API key
4. Open the project in VS Code and click "Reopen in Container" when prompted (or run the "Remote-Containers: Reopen in Container" command)
5. Once inside the container, build and run Vibesh:
   ```bash
   go build -o vibesh
   ./vibesh
   ```
   Or use the provided script:
   ```bash
   ./run.sh
   ```

### Available commands

- `exit` - Exit the shell
- `mode` - Switch between processing modes (direct, ai, rag)
- `history` - Display command history
- `context` - Show current directory context information
- `help` - Display help information

### Directory Context Feature

Vibesh automatically provides the AI with context about your current directory when processing commands in `ai` or `rag` modes. This helps the AI generate more relevant commands based on your current environment.

You can view this context information at any time by typing `context`.

### Example usage

```
Vibesh - AI-Enhanced Interactive Shell
Type 'exit' to quit, 'mode' to switch processing mode, 'help' for available commands
Modes: 'direct' (default), 'ai', 'rag'

vibesh(direct)> ls -la
[Output of ls -la command]

vibesh(direct)> mode
Current mode: direct
Available modes: direct, ai, rag
Select mode: ai
Mode switched to: ai

vibesh(ai)> show me all text files
[AI] Interpreted 'show me all text files' as:
find . -name "*.txt"

Output:
[Output of the find command]

vibesh(ai)> context
Current directory: /Users/username/projects/vibesh

Contents:
- [FILE] main.go (8254 bytes)
- [FILE] go.mod (82 bytes)
- [FILE] go.sum (215 bytes)
- [DIR] .devcontainer
- [FILE] README.md (2541 bytes)
...

Summary: 2 directories, 8 files

vibesh(ai)> help

Vibesh Help:
---------------
Built-in commands:
  exit     - Exit the shell
  mode     - Switch between processing modes
  history  - Display command history
  context  - Show current directory context
  help     - Display this help message

Modes:
  direct   - Commands are executed directly in the shell
  ai       - Natural language is converted to shell commands using AI
  rag      - Commands are matched against a knowledge base with AI fallback

## How it works

1. **Direct mode**: Commands are passed directly to a shell.
2. **AI mode**: Uses OpenAI to convert natural language into shell commands, then executes them.
3. **RAG mode**: Attempts to match your request against a knowledge base of common commands. If no match is found, falls back to AI processing.

The AI modes include your current directory context to help generate more relevant commands based on your environment.

## Extending

To add more commands to the RAG knowledge base, modify the `NewRAGProcessor` function in `main.go`.
