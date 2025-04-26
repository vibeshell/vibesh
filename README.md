# VibeSH 🌊✨

VibeSH isn't just a shell—it's your AI-enhanced command line **vibe check**. Turn natural language into powerful shell commands and experience a new level of productivity that feels almost telepathic.

Amplify your sysadmin powers by 1000x with an interactive shell that lets you execute commands in different vibes:

1. **Direct mode** - Traditional shell experience with raw command execution
2. **AI mode** - Speak your intent in natural language and watch it transform into precise shell commands
3. **RAG mode** - Instant command matching against a curated knowledge base of sysadmin wisdom
4. **AI-YOLO mode** - For the fearless: commands execute instantly without confirmation
5. **RAG-YOLO mode** - Maximum velocity with knowledge-backed commands that execute immediately

## Why VibeSH? ⚡

* **1000x Productivity** - Stop memorizing complex syntax and flags; just describe what you want to accomplish
* **Contextual Awareness** - VibeSH understands your environment and tailors commands to your system
* **Risk Protection** - Color-coded risk assessment keeps you informed about command impact
* **Supernatural Speed** - YOLO modes for trusted environments where productivity matters most
* **Learning Tool** - See the exact commands behind your natural language requests, leveling up your CLI skills

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

### Running VibeSH

#### Interactive Shell

```bash
./vibesh
```

#### Script Execution

VibeSH can execute commands from script files:

```bash
vibesh script_file.vsh
```

#### Shebang Support

You can use VibeSH as an interpreter in scripts with a shebang:

```bash
#!/usr/bin/env vibesh

# Count files in the current directory
count the number of files in the current directory

# List all Go files
find all Go files in this directory
```

Make your script executable with `chmod +x your_script.vsh` and run it directly.

#### Piped Input

You can also pipe commands to VibeSH:

```bash
echo "list all text files" | vibesh
```

### Using with DevContainer

VibeSH can be run inside a DevContainer for an isolated development environment:

1. Install [Docker](https://www.docker.com/get-started/) and [VS Code](https://code.visualstudio.com/)
2. Install the [Remote - Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension in VS Code
3. Copy `devcontainer.env.template` to `.env.devcontainer` and add your OpenAI API key
4. Open the project in VS Code and click "Reopen in Container" when prompted (or run the "Remote-Containers: Reopen in Container" command)
5. Once inside the container, build and run VibeSH:
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
- `mode [mode_name]` - Switch processing mode. Without an argument, it prompts for mode selection. With an argument, directly switches to the specified mode (e.g., `mode ai`)
- `history` - Display command history
- `context` - Show current directory context information
- `help` - Display help information

### AI Command Risk Assessment

VibeSH provides automatic risk assessment for AI-generated commands:

- **Risk Score**: Commands are rated on a scale from 0-10 based on potential for data loss or system impact
- **Read/Write Classification**: Commands are identified as reading from or writing to disk
- **Color Coding**:
  - Green (0-3): Low risk
  - Yellow (4-6): Medium risk
  - Red (7-10): High risk

High-risk commands (score ≥ 7) will trigger a confirmation prompt before execution, allowing you to review and approve potentially dangerous operations.

### "Intelligent" Risk Management

VibeSH now intelligently manages command execution risk:

1. Low-risk commands (0-3) execute immediately with visual risk indication
2. Medium-risk commands (4-6) execute with visual warnings
3. High-risk commands (7-10) require explicit confirmation before execution
4. Commands in YOLO modes bypass confirmation regardless of risk level

The shell provides clear feedback about command risk through color-coding and detailed risk information, helping you make informed decisions about command execution.

### YOLO Mode

YOLO ("You Only Live Once") modes execute commands directly without showing you what they are first. When using the AI or RAG processors in YOLO mode:

- The prompt is shown in red to indicate you're in a potentially dangerous mode
- Commands are executed immediately without confirmation
- The output shows what command was run after execution

⚠️ **CAUTION:** YOLO modes should be used with care, as they execute commands without giving you a chance to review them first, even for high-risk operations.

### Directory Context Feature

VibeSH automatically provides the AI with context about your current directory when processing commands in `ai` or `rag` modes. This helps the AI generate more relevant commands based on your current environment.

You can view this context information at any time by typing `context`.

## Real Sysadmin Superpowers 🚀

VibeSH transforms everyday tasks into supernatural experiences:

```
vibesh(ai)> show me the largest files in this directory and sort them

[AI] I'll find the largest files in the current directory and sort them by size.
Risk: 1/10 | Read: true | Write: false
Command: find . -type f -exec du -sh {} \; | sort -rh | head -n 10

12M   ./vibesh
5.2M  ./vibesh.old
1.2M  ./.git/objects/pack/pack-f83jd73.pack
...

vibesh(ai)> find all processes using more than 1GB of memory

[AI] I'll show processes consuming more than 1GB of RAM.
Risk: 1/10 | Read: true | Write: false
Command: ps aux | awk '{if ($6 > 1000000) print $0}'

USER    PID  %CPU %MEM    VSZ   RSS TTY   STAT START   TIME COMMAND
martin  1234  12.0 14.3 5821680 2343628 ?  Sl   08:22   2:14 /usr/bin/firefox
...
```

Beyond just executing commands, VibeSH becomes your sysadmin copilot:

```
vibesh(ai)> help me debug why my web server isn't responding

[AI] I'll check if your web server is running and identify common issues.
Risk: 2/10 | Read: true | Write: false
Command: (ps aux | grep -E 'apache|nginx|httpd') && netstat -tulpn | grep -E '80|443' && curl -I localhost

...
```

## How it works

1. **Direct mode**: Commands are passed directly to a shell.
2. **AI mode**: Uses OpenAI to convert natural language into shell commands, then executes them.
3. **RAG mode**: Attempts to match your request against a knowledge base of common commands. If no match is found, falls back to AI processing.
4. **YOLO modes**: The AI/RAG-YOLO modes execute commands immediately without showing you the command first.

### OpenAI Function Calling API

VibeSH leverages OpenAI's Function Calling API to ensure well-structured, reliable command generation:

1. Your natural language request is sent to the OpenAI API
2. A specially designed function schema instructs the AI model to:
   - Generate a proper command array
   - Provide a friendly explanation
   - Assess risk level (0-10)
   - Identify read/write operations
3. The JSON response follows a strict schema, ensuring consistent parsing and risk assessment
4. VibeSH validates and executes the command based on risk level and mode

This approach provides several advantages:
- More reliable parsing of AI responses
- Consistent risk assessment
- Better structured command generation
- Improved error handling

The structured JSON responses contain:
- A friendly explanation of what the command will do
- The exact command broken down into executable and arguments
- Risk assessment scoring
- Classification of whether the command reads or writes data

This structured approach allows VibeSH to provide better safety information and more accurate command execution, while giving you control over how commands are confirmed based on their risk level.

## Script Writing Guide

When writing VibeSH scripts:

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

## The VibeSH Philosophy

VibeSH represents a new paradigm in command-line interfaces—one where technology adapts to humans, not the other way around. Through the power of AI, we're creating tools that understand your intent, protect you from mistakes, and multiply your capabilities exponentially.

VibeSH is more than a shell; it's a glimpse into a future where the boundary between what you want and what the computer does begins to dissolve. Now that's a vibe worth sharing. ✨
