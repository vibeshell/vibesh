package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// CommandProcessor handles different ways of processing commands
type CommandProcessor interface {
	Process(command string, history []string) (string, error)
}

// DirectShellProcessor executes commands directly in the shell
type DirectShellProcessor struct{}

func (p *DirectShellProcessor) Process(command string, history []string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// AIProcessor represents a processor that uses AI to interpret commands
type AIProcessor struct {
	client *openai.Client
	yolo   bool
}

func NewAIProcessor(apiKey string, yolo bool) *AIProcessor {
	return &AIProcessor{
		client: openai.NewClient(apiKey),
		yolo:   yolo,
	}
}

// getDirectoryContext gets information about the current directory for context
func getDirectoryContext() string {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "Error getting current directory"
	}

	// List files in the current directory
	files, err := os.ReadDir(".")
	if err != nil {
		return fmt.Sprintf("Current directory: %s\nError listing files", cwd)
	}

	// Format directory contents
	var fileList strings.Builder
	dirCount := 0
	fileCount := 0

	fileList.WriteString(fmt.Sprintf("Current directory: %s\n\nContents:\n", cwd))
	for _, file := range files {
		if file.IsDir() {
			dirCount++
			fileList.WriteString(fmt.Sprintf("- [DIR] %s\n", file.Name()))
		} else {
			fileCount++
			// Only include first 20 files to avoid too much content
			if fileCount <= 20 {
				info, _ := file.Info()
				size := info.Size()
				fileList.WriteString(fmt.Sprintf("- [FILE] %s (%d bytes)\n", file.Name(), size))
			}
		}
	}

	if fileCount > 20 {
		fileList.WriteString(fmt.Sprintf("... and %d more files\n", fileCount-20))
	}

	summary := fmt.Sprintf("\nSummary: %d directories, %d files\n", dirCount, fileCount)
	fileList.WriteString(summary)

	return fileList.String()
}

func (p *AIProcessor) Process(command string, history []string) (string, error) {
	// Skip AI processing if the client is nil (no API key)
	if p.client == nil {
		return "[AI] API key not set. Please set OPENAI_API_KEY environment variable.", nil
	}

	// Create a context message history from the command history
	var messages []openai.ChatCompletionMessage

	// Get directory context
	dirContext := getDirectoryContext()

	// System message describing what we want - updated to use the new format
	systemPrompt := `You are VibeSH, an AI-powered natural-language shell assistant. When the user gives an instruction, you **do not** execute anything yourself. Instead:

1. Interpret the user's intent.
2. Determine the single most appropriate shell command (as an executable plus arguments) to fulfill it.
3. Evaluate:
   - **risk_score**: integer 0–10 based on potential data loss or system impact
   - **does_read**: true if the command reads files or system state
   - **does_write**: true if it creates, modifies, or deletes files or data
4. Respond **only** with a JSON object in this exact schema (no extra fields, no comments, no prose outside the JSON):

{
  "reply": "string",         // One friendly sentence of what you will do
  "cmd": ["string", "..."],  // Array: ["executable", "arg1", "arg2", …]
  "risk_score": number,      // 0 (no risk) to 10 (extremely risky)
  "does_read": boolean,      // true if it reads from disk, network, etc.
  "does_write": boolean      // true if it writes/modifies/deletes data
}`

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: systemPrompt,
	})

	// Add directory context
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: "Directory context:\n" + dirContext,
	})

	// Add history for context
	for _, cmd := range history {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: cmd,
		})
	}

	// Add the current command
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: command,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := p.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
		},
	)

	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %v", err)
	}

	aiResponseText := resp.Choices[0].Message.Content

	// Parse the JSON response
	var aiResponse AICommandResponse
	err = json.Unmarshal([]byte(aiResponseText), &aiResponse)
	if err != nil {
		return fmt.Sprintf("[AI] Error parsing AI response: %v\nRaw response: %s", err, aiResponseText), nil
	}

	// Build the shell command from the cmd array
	shellCmd := strings.Join(aiResponse.Cmd, " ")

	// Color formatting based on risk score
	var riskColor string
	if aiResponse.RiskScore >= 7 {
		riskColor = "\033[1;31m" // Red for high risk
	} else if aiResponse.RiskScore >= 4 {
		riskColor = "\033[1;33m" // Yellow for medium risk
	} else {
		riskColor = "\033[1;32m" // Green for low risk
	}

	riskInfo := fmt.Sprintf("%sRisk: %d/10%s | Read: %v | Write: %v",
		riskColor, aiResponse.RiskScore, "\033[0m", aiResponse.DoesRead, aiResponse.DoesWrite)

	if p.yolo {
		// In YOLO mode, just execute the command directly
		cmd := exec.Command(aiResponse.Cmd[0], aiResponse.Cmd[1:]...)
		output, err := cmd.CombinedOutput()

		result := fmt.Sprintf("[AI YOLO] %s\n%s\nRunning: %s\n\nOutput:\n%s",
			aiResponse.Reply, riskInfo, shellCmd, string(output))

		if err != nil {
			result += fmt.Sprintf("\nError: %v", err)
		}

		return result, nil
	} else {
		// Regular mode - show the command and ask for confirmation
		shellExecCmd := exec.Command(aiResponse.Cmd[0], aiResponse.Cmd[1:]...)
		shellOutput, shellErr := shellExecCmd.CombinedOutput()

		result := fmt.Sprintf("[AI] %s\n%s\nCommand: %s\n\nOutput:\n%s",
			aiResponse.Reply, riskInfo, shellCmd, string(shellOutput))

		if shellErr != nil {
			result += fmt.Sprintf("\nError: %v", shellErr)
		}

		return result, nil
	}
}

// RAGProcessor represents a processor that uses retrieval-augmented generation
type RAGProcessor struct {
	client *openai.Client
	// Simple in-memory knowledge base for command examples
	knowledgeBase map[string]string
	yolo          bool
}

func NewRAGProcessor(apiKey string, yolo bool) *RAGProcessor {
	// Initialize with some sample commands
	kb := map[string]string{
		// General file system commands
		"list files":         "ls -la",
		"show hidden files":  "ls -la | grep '^\\.'",
		"find file":          "find . -name",
		"find text in files": "grep -r 'text' .",
		"create directory":   "mkdir -p",
		"remove directory":   "rm -rf",
		"copy file":          "cp",
		"move file":          "mv",
		"change permissions": "chmod",
		"change owner":       "chown",

		// System information
		"show disk space":           "df -h",
		"check memory":              "free -m || vm_stat",
		"show running processes":    "ps aux",
		"show system info":          "uname -a",
		"find large files":          "find . -type f -size +100M",
		"check network connections": "netstat -tuln || lsof -i -P -n",
		"show ip address":           "ifconfig || ip addr",
		"monitor cpu usage":         "top",
		"check system logs":         "tail -f /var/log/syslog || tail -f /var/log/system.log",

		// Mac-specific commands
		"show mac info":     "system_profiler SPHardwareDataType",
		"list applications": "ls -la /Applications",
		"show mac version":  "sw_vers",
		"flush dns":         "dscacheutil -flushcache; killall -HUP mDNSResponder",
		"show network info": "networksetup -listallhardwareports",
		"show battery info": "pmset -g batt",

		// Development-related commands
		"list ports":             "lsof -i -P -n | grep LISTEN",
		"kill process on port":   "lsof -ti tcp:PORT | xargs kill",
		"check git status":       "git status",
		"git pull":               "git pull",
		"git push":               "git push",
		"list docker containers": "docker ps",
		"build docker image":     "docker build -t NAME .",
		"run docker container":   "docker run -it --rm NAME",
		"go build":               "go build",
		"go test":                "go test ./...",
		"go run":                 "go run main.go",
		"npm install":            "npm install",
		"npm start":              "npm start",
		"view json pretty":       "cat FILE | jq",
	}

	return &RAGProcessor{
		client:        openai.NewClient(apiKey),
		knowledgeBase: kb,
		yolo:          yolo,
	}
}

func (p *RAGProcessor) findSimilarCommand(query string) (string, bool) {
	// Simple implementation: check if any key contains words from the query
	queryWords := strings.Fields(strings.ToLower(query))

	bestMatchKey := ""
	bestMatchCount := 0

	for key := range p.knowledgeBase {
		matchCount := 0
		for _, word := range queryWords {
			if strings.Contains(strings.ToLower(key), word) {
				matchCount++
			}
		}

		if matchCount > bestMatchCount {
			bestMatchCount = matchCount
			bestMatchKey = key
		}
	}

	if bestMatchCount > 0 {
		return p.knowledgeBase[bestMatchKey], true
	}

	return "", false
}

func (p *RAGProcessor) Process(command string, history []string) (string, error) {
	// Try to find a similar command in the knowledge base
	if shellCmd, found := p.findSimilarCommand(command); found {
		if p.yolo {
			// In YOLO mode, just execute without showing the matched command first
			cmd := exec.Command("sh", "-c", shellCmd)
			output, err := cmd.CombinedOutput()

			result := fmt.Sprintf("[RAG YOLO] Running: %s\n\nOutput:\n%s",
				shellCmd, string(output))

			if err != nil {
				result += fmt.Sprintf("\nError: %v", err)
			}

			return result, nil
		} else {
			// Regular mode - show the command and execute
			cmd := exec.Command("sh", "-c", shellCmd)
			output, err := cmd.CombinedOutput()

			result := fmt.Sprintf("[RAG] Matched '%s' to command: %s\n\nOutput:\n%s",
				command, shellCmd, string(output))

			if err != nil {
				result += fmt.Sprintf("\nError: %v", err)
			}

		return result, nil
	}

	// If not found in knowledge base and we have a client, fall back to AI
	if p.client != nil {
		aiProcessor := AIProcessor{client: p.client}
		return aiProcessor.Process(command, history)
	}

	return "[RAG] No matching command found and AI fallback not available.", nil
}

// processScriptFile reads and executes commands from a script file
func processScriptFile(filename string, processor CommandProcessor) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open script file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Skip the first line if it's a shebang
	if scanner.Scan() {
		firstLine := scanner.Text()
		if !strings.HasPrefix(firstLine, "#!") {
			// If it's not a shebang, process it as a command
			output, err := processor.Process(firstLine, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
			} else {
				fmt.Println(output)
			}
		}
	}

	// Process the rest of the file
	var history []string
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Add to history for context
		history = append(history, line)

		// Process the command
		output, err := processor.Process(line, history[:len(history)-1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		} else {
			fmt.Println(output)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading script file: %v", err)
	}

	return nil
}

func main() {
	// Get OpenAI API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")

	// Create processors
	aiProcessor := NewAIProcessor(apiKey, false)
	ragProcessor := NewRAGProcessor(apiKey, false)
	directProcessor := &DirectShellProcessor{}

	// Check if a script file is provided as an argument
	if len(os.Args) > 1 {
		scriptFile := os.Args[1]

		// Determine which processor to use based on script extension or content
		// For simplicity, we'll use the AI processor by default for scripts
		err := processScriptFile(scriptFile, aiProcessor)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Script execution failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// No script file, start interactive mode
	fmt.Println("Vibesh - AI-Enhanced Interactive Shell")
	fmt.Println("Type 'exit' to quit, 'mode' to switch processing mode, 'help' for available commands")
	fmt.Println("Modes: 'direct' (default), 'ai', 'rag'")

	if apiKey == "" {
		fmt.Println("Warning: OPENAI_API_KEY not set. AI and RAG modes will have limited functionality.")
	}

	reader := bufio.NewReader(os.Stdin)
	var commandHistory []string

	processors := map[string]CommandProcessor{
		"direct": directProcessor,
		"ai":     aiProcessor,
		"rag":    ragProcessor,
	}

	currentMode := "direct"

	// Check for piped input - non-interactive mode
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data is being piped in
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			command := scanner.Text()
			processor := processors[currentMode]
			output, err := processor.Process(command, commandHistory)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error executing command:", err)
			} else {
				fmt.Println(output)
			}
			commandHistory = append(commandHistory, command)
		}
		os.Exit(0)
	}

	// Interactive mode
	for {
		fmt.Printf("\033[1;32mvibesh(%s)>\033[0m ", currentMode)

		input, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nGoodbye!")
				break
			}
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
			continue
		}

		// Trim the input and handle empty commands
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Add command to history
		commandHistory = append(commandHistory, input)

		// Handle special commands
		if input == "exit" {
			fmt.Println("Goodbye!")
			break
		}

		if input == "help" {
			printHelp(currentMode)
			continue
		}

		if input == "mode" {
			fmt.Printf("Current mode: %s\nAvailable modes: direct, ai, rag\n", currentMode)
			fmt.Print("Select mode: ")
			modeInput, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error reading mode:", err)
				continue
			}

			modeInput = strings.TrimSpace(modeInput)
			if _, ok := processors[modeInput]; ok {
				currentMode = modeInput
				fmt.Printf("Mode switched to: %s\n", currentMode)
			} else {
				fmt.Printf("Invalid mode: %s\n", modeInput)
			}
			continue
		}

		if input == "history" {
			fmt.Println("Command history:")
			for i, cmd := range commandHistory[:len(commandHistory)-1] {
				fmt.Printf("%d: %s\n", i+1, cmd)
			}
			continue
		}

		if input == "context" {
			fmt.Println(getDirectoryContext())
			continue
		}

		// Process the command using the selected processor
		processor := processors[currentMode]
		output, err := processor.Process(input, commandHistory[:len(commandHistory)-1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error executing command:", err)
			continue
		}

		fmt.Println(output)
	}
}

// printHelp displays help information based on the current mode
func printHelp(mode string) {
	fmt.Println("\nVibesh Help:")
	fmt.Println("---------------")
	fmt.Println("Built-in commands:")
	fmt.Println("  exit     - Exit the shell")
	fmt.Println("  mode     - Switch between processing modes")
	fmt.Println("  history  - Display command history")
	fmt.Println("  context  - Show current directory context")
	fmt.Println("  help     - Display this help message")
	fmt.Println("\nModes:")
	fmt.Println("  direct   - Commands are executed directly in the shell")
	fmt.Println("  ai       - Natural language is converted to shell commands using AI")
	fmt.Println("  rag      - Commands are matched against a knowledge base with AI fallback")

	if strings.HasPrefix(mode, "rag") {
		fmt.Println("\nPopular RAG Commands:")
		fmt.Println("  list files               - List files in the current directory")
		fmt.Println("  find file                - Find a file by name")
		fmt.Println("  find text in files       - Search for text in files")
		fmt.Println("  show disk space          - Show disk usage")
		fmt.Println("  check git status         - Check git repository status")
		fmt.Println("  list ports               - List open network ports")
		fmt.Println("  show system info         - Display system information")
	}

	fmt.Println()
}
