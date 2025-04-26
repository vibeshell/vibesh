# Vibesh

Vibesh is an AI-enhanced interactive shell that allows you to execute commands in different modes:

1. **Direct mode** - Commands are executed directly in the underlying shell
2. **AI mode** - Natural language is converted to shell commands using AI
3. **RAG mode** - Commands are matched against a knowledge base of common commands
4. **AI-YOLO mode** - Like AI mode but executes commands directly without confirmation
5. **RAG-YOLO mode** - Like RAG mode but executes commands directly without confirmation

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

After building, make sure to place the `vibesh` executable in your PATH to enable script execution.

## Usage

### Setting up your API key

For AI and RAG modes to work with full functionality, set your OpenAI API key:

```bash
export OPENAI_API_KEY=your_openai_api_key_here
```

### Running Vibesh

#### Interactive Shell

```bash
./vibesh
```

#### Script Execution

Vibesh can execute commands from script files:

```bash
vibesh script_file.vsh
```

#### Shebang Support

You can use Vibesh as an interpreter in scripts with a shebang:

```bash
#!/usr/bin/env vibesh

# Count files in the current directory
count the number of files in the current directory

# List all Go files
find all Go files in this directory
```

Make your script executable with `chmod +x your_script.vsh` and run it directly.

#### Piped Input

You can also pipe commands to Vibesh:

```bash
echo "list all text files" | vibesh
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
- `mode` - Switch between processing modes (direct, ai, rag, ai-yolo, rag-yolo)
- `history` - Display command history
- `context` - Show current directory context information
- `help` - Display help information

### AI Command Risk Assessment

Vibesh now includes risk assessment for AI-generated commands:

- **Risk Score**: Commands are rated on a scale from 0-10 based on potential for data loss or system impact
- **Read/Write Classification**: Commands are identified as reading from or writing to disk
- **Color Coding**:
  - Green (0-3): Low risk
  - Yellow (4-6): Medium risk
  - Red (7-10): High risk

This helps you quickly identify potentially dangerous commands before execution.

### YOLO Mode

YOLO ("You Only Live Once") modes execute commands directly without showing you what they are first. When using the AI or RAG processors in YOLO mode:

- The prompt is shown in red to indicate you're in a potentially dangerous mode
- Commands are executed immediately without confirmation
- The output shows what command was run after execution

⚠️ **CAUTION:** YOLO modes should be used with care, as they execute commands without giving you a chance to review them first.

### Directory Context Feature

Vibesh automatically provides the AI with context about your current directory when processing commands in `ai` or `rag` modes. This helps the AI generate more relevant commands based on your current environment.

You can view this context information at any time by typing `context`.

### Example usage

```
Vibesh - AI-Enhanced Interactive Shell
Type 'exit' to quit, 'mode' to switch processing mode, 'help' for available commands
Modes: 'direct' (default), 'ai', 'rag', 'ai-yolo', 'rag-yolo'

vibesh(direct)> ls -la
[Output of ls -la command]

vibesh(direct)> mode
Current mode: direct
Available modes: direct, ai, rag, ai-yolo, rag-yolo
Select mode: ai
Mode switched to: ai

vibesh(ai)> show me all Go files
[AI] I'll find all Go files in the current directory and its subdirectories.
Risk: 1/10 | Read: true | Write: false
Command: find . -name "*.go"

Output:
./main.go

vibesh(ai)> mode
Current mode: ai
Available modes: direct, ai, rag, ai-yolo, rag-yolo
Select mode: ai-yolo
Mode switched to: ai-yolo
⚠️  CAUTION: YOLO MODE EXECUTES COMMANDS WITHOUT CONFIRMATION

vibesh(ai-yolo)> delete all temporary files
[AI YOLO] I'll remove all temporary files in the current directory.
Risk: 7/10 | Read: true | Write: true
Running: find . -name "*~" -o -name "*.tmp" -delete

Output:
[Command output]
```

## How it works

1. **Direct mode**: Commands are passed directly to a shell.
2. **AI mode**: Uses OpenAI to convert natural language into shell commands, then executes them.
3. **RAG mode**: Attempts to match your request against a knowledge base of common commands. If no match is found, falls back to AI processing.
4. **YOLO modes**: The AI/RAG-YOLO modes execute commands immediately without showing you the command first.

The AI generates structured responses with:
- A friendly explanation of what the command will do
- The exact command broken down into executable and arguments
- Risk assessment scoring
- Classification of whether the command reads or writes data

This structured approach allows Vibesh to provide better safety information and more accurate command execution.

## Script Writing Guide

When writing Vibesh scripts:

1. Start with the shebang: `#!/usr/bin/env vibesh`
2. Use comments with `#` at the start of the line
3. Write commands in natural language
4. Commands are executed sequentially
5. Command history is maintained between commands, so context is preserved
6. Empty lines and comments are ignored

Example script:
```bash
#!/usr/bin/env vibesh

# Get system information
show me system info

# Export variable for later use
export count=`find . -type f | wc -l`

# Use the variable
echo "This directory contains $count files"
```

## Extending

To add more commands to the RAG knowledge base, modify the `NewRAGProcessor` function in `main.go`.
