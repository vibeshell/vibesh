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

// AIResponse represents the structured response from the AI
type AIResponse struct {
	Reply     string   `json:"reply"`      // Friendly explanation of what the command will do
	Cmd       []string `json:"cmd"`        // Array of command and arguments
	RiskScore int      `json:"risk_score"` // Risk score from 0-10
	DoesRead  bool     `json:"does_read"`  // Whether the command reads from disk
	DoesWrite bool     `json:"does_write"` // Whether the command writes to disk
}

// AIProcessor represents a processor that uses AI to interpret commands
type AIProcessor struct {
	client *openai.Client
	yolo   bool // Whether to execute commands without confirmation
}

func NewAIProcessor(apiKey string) *AIProcessor {
	return &AIProcessor{
		client: openai.NewClient(apiKey),
		yolo:   false,
	}
}

// NewAIYoloProcessor creates an AI processor that executes commands without confirmation
func NewAIYoloProcessor(apiKey string) *AIProcessor {
	return &AIProcessor{
		client: openai.NewClient(apiKey),
		yolo:   true,
	}
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

	// System message describing what we want - updated to prompt for JSON
	systemPrompt := `You are ShellAI, an AI-powered natural-language shell assistant. When the user gives an instruction, you **do not** execute anything yourself. Instead:

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

	// Setup JSON response format with function calling
	functions := []openai.FunctionDefinition{
		{
			Name:        "generate_shell_command",
			Description: "Generate a shell command based on user input",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"reply": map[string]interface{}{
						"type":        "string",
						"description": "One friendly sentence of what you will do",
					},
					"cmd": map[string]interface{}{
						"type":        "array",
						"description": "Array: [\"executable\", \"arg1\", \"arg2\", …]",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"risk_score": map[string]interface{}{
						"type":        "integer",
						"description": "0 (no risk) to 10 (extremely risky)",
						"minimum":     0,
						"maximum":     10,
					},
					"does_read": map[string]interface{}{
						"type":        "boolean",
						"description": "true if it reads from disk, network, etc.",
					},
					"does_write": map[string]interface{}{
						"type":        "boolean",
						"description": "true if it writes/modifies/deletes data",
					},
				},
				"required": []string{"reply", "cmd", "risk_score", "does_read", "does_write"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := p.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model:       openai.GPT3Dot5Turbo,
			Messages:    messages,
			Functions:   functions,
			FunctionCall: openai.FunctionCall{
				Name: "generate_shell_command",
			},
		},
	)

	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %v", err)
	}

	// Extract the function call response
	functionCallResponse := resp.Choices[0].Message.FunctionCall

	// Parse the JSON response from the function call
	var aiResponse AIResponse
	if err := json.Unmarshal([]byte(functionCallResponse.Arguments), &aiResponse); err != nil {
		return "", fmt.Errorf("Failed to parse AI response as JSON: %v\nRaw response: %s", err, functionCallResponse.Arguments)
	}

	// Convert the command array to a shell command string
	shellCmdString := strings.Join(aiResponse.Cmd, " ")

	// Get risk color based on risk score
	riskColor := getRiskColor(aiResponse.RiskScore)

	// Determine if we should ask for confirmation based on risk score and YOLO mode
	shouldConfirm := aiResponse.RiskScore >= 7 && !p.yolo

	// Format result with risk information
	formatTags := []string{
		fmt.Sprintf("Risk: %s%d/10\033[0m", riskColor, aiResponse.RiskScore),
		fmt.Sprintf("Read: %v", aiResponse.DoesRead),
		fmt.Sprintf("Write: %v", aiResponse.DoesWrite),
	}

	// Build the output
	var result strings.Builder

	// Add mode prefix with YOLO warning if applicable
	if p.yolo {
		result.WriteString("[AI YOLO] ")
	} else {
		result.WriteString("[AI] ")
	}

	// Add the friendly explanation
	result.WriteString(aiResponse.Reply)
	result.WriteString("\n")

	// Add the risk information
	result.WriteString(strings.Join(formatTags, " | "))
	result.WriteString("\n")

	// Check if we need confirmation
	if shouldConfirm {
		result.WriteString(fmt.Sprintf("\033[1;31mWARNING: This command has a high risk score (%d/10).\033[0m\n", aiResponse.RiskScore))
		result.WriteString(fmt.Sprintf("Command: %s\n\n", shellCmdString))
		result.WriteString("Do you want to execute this command? (y/n): ")

		// Print the current result and get user confirmation
		fmt.Print(result.String())
		reader := bufio.NewReader(os.Stdin)
		confirm, _ := reader.ReadString('\n')
		confirm = strings.TrimSpace(confirm)

		// Reset the result for the final output
		result.Reset()

		if strings.ToLower(confirm) != "y" {
			return "Command execution cancelled by user.", nil
		}

		// Rebuild the prefix for the final output
		if p.yolo {
			result.WriteString("[AI YOLO] ")
		} else {
			result.WriteString("[AI] ")
		}
	}

	// Execute the command
	if p.yolo {
		result.WriteString(fmt.Sprintf("Running: %s\n\n", shellCmdString))
	} else {
		result.WriteString(fmt.Sprintf("Command: %s\n\n", shellCmdString))
	}

	// Execute the command
	cmd := exec.Command("sh", "-c", shellCmdString)
	shellOutput, shellErr := cmd.CombinedOutput()

	// Add the command output
	result.WriteString(string(shellOutput))

	// Add any error information
	if shellErr != nil {
		result.WriteString(fmt.Sprintf("\nError: %v", shellErr))
	}

	return result.String(), nil
}

// getRiskColor returns ANSI color code based on the risk score
func getRiskColor(risk int) string {
	if risk <= 3 {
		return "\033[1;32m" // Green for low risk
	} else if risk <= 6 {
		return "\033[1;33m" // Yellow for medium risk
	} else {
		return "\033[1;31m" // Red for high risk
	}
}

// RAGProcessor represents a processor that uses retrieval-augmented generation
type RAGProcessor struct {
	client *openai.Client
	// Simple in-memory knowledge base for command examples
	knowledgeBase map[string]string
	yolo          bool // Whether to execute commands without confirmation
}

func NewRAGProcessor(apiKey string) *RAGProcessor {
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
		yolo:          false,
	}
}

// NewRAGYoloProcessor creates a RAG processor that executes commands without confirmation
func NewRAGYoloProcessor(apiKey string) *RAGProcessor {
	processor := NewRAGProcessor(apiKey)
	processor.yolo = true
	return processor
}

// Assign estimated risk scores to common RAG commands
func getRAGCommandRisk(cmd string) (int, bool, bool) {
	// Default values
	risk := 3 // Medium-low risk by default
	doesRead := true
	doesWrite := false

	// High-risk commands
	if strings.HasPrefix(cmd, "rm -rf") {
		risk = 9
		doesWrite = true
	} else if strings.HasPrefix(cmd, "chmod") || strings.HasPrefix(cmd, "chown") {
		risk = 7
		doesWrite = true
	}

	// Medium-risk commands
	if strings.HasPrefix(cmd, "git push") || strings.HasPrefix(cmd, "git pull") {
		risk = 5
		doesWrite = true
	} else if strings.HasPrefix(cmd, "mv") || strings.HasPrefix(cmd, "cp") {
		risk = 5
		doesWrite = true
	} else if strings.HasPrefix(cmd, "mkdir") {
		risk = 4
		doesWrite = true
	}

	// Low-risk commands (read-only)
	if strings.HasPrefix(cmd, "ls") || strings.HasPrefix(cmd, "find") ||
	   strings.HasPrefix(cmd, "grep") || strings.HasPrefix(cmd, "df") ||
	   strings.HasPrefix(cmd, "ps") || strings.HasPrefix(cmd, "uname") ||
	   strings.HasPrefix(cmd, "git status") {
		risk = 1
		doesWrite = false
	}

	return risk, doesRead, doesWrite
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
		// Get risk assessment for this command
		riskScore, doesRead, doesWrite := getRAGCommandRisk(shellCmd)

		// Get risk color
		riskColor := getRiskColor(riskScore)

		// Determine if we should ask for confirmation
		shouldConfirm := riskScore >= 7 && !p.yolo

		// Format result with risk information
		formatTags := []string{
			fmt.Sprintf("Risk: %s%d/10\033[0m", riskColor, riskScore),
			fmt.Sprintf("Read: %v", doesRead),
			fmt.Sprintf("Write: %v", doesWrite),
		}

		// Build the output
		var result strings.Builder

		// Add mode prefix with YOLO warning if applicable
		if p.yolo {
			result.WriteString("[RAG YOLO] ")
		} else {
			result.WriteString("[RAG] ")
		}

		// Add matched information
		result.WriteString(fmt.Sprintf("Matched '%s' to command: %s\n", command, shellCmd))

		// Add the risk information
		result.WriteString(strings.Join(formatTags, " | "))
		result.WriteString("\n")

		// Check if we need confirmation
		if shouldConfirm {
			result.WriteString(fmt.Sprintf("\033[1;31mWARNING: This command has a high risk score (%d/10).\033[0m\n", riskScore))
			result.WriteString("Do you want to execute this command? (y/n): ")

			// Print the current result and get user confirmation
			fmt.Print(result.String())
			reader := bufio.NewReader(os.Stdin)
			confirm, _ := reader.ReadString('\n')
			confirm = strings.TrimSpace(confirm)

			// Reset the result for the final output
			result.Reset()

			if strings.ToLower(confirm) != "y" {
				return "Command execution cancelled by user.", nil
			}

			// Rebuild the prefix for the final output
			if p.yolo {
				result.WriteString("[RAG YOLO] ")
			} else {
				result.WriteString("[RAG] ")
			}
			result.WriteString(fmt.Sprintf("Matched '%s' to command: %s\n", command, shellCmd))
		}

		// Execute the command
		cmd := exec.Command("sh", "-c", shellCmd)
		output, err := cmd.CombinedOutput()

		// Add output information
		result.WriteString("\nOutput:\n")
		result.WriteString(string(output))

		// Add error information if any
		if err != nil {
			result.WriteString(fmt.Sprintf("\nError: %v", err))
		}

		return result.String(), nil
	}

	// If not found in knowledge base and we have a client, fall back to AI
	if p.client != nil {
		aiProcessor := AIProcessor{client: p.client, yolo: p.yolo}
		return aiProcessor.Process(command, history)
	}

	return "[RAG] No matching command found and AI fallback not available.", nil
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
	aiProcessor := NewAIProcessor(apiKey)
	aiYoloProcessor := NewAIYoloProcessor(apiKey)
	ragProcessor := NewRAGProcessor(apiKey)
	ragYoloProcessor := NewRAGYoloProcessor(apiKey)
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
	fmt.Println("Modes: 'direct' (default), 'ai', 'rag', 'ai-yolo', 'rag-yolo'")

	if apiKey == "" {
		fmt.Println("Warning: OPENAI_API_KEY not set. AI and RAG modes will have limited functionality.")
	}

	reader := bufio.NewReader(os.Stdin)
	var commandHistory []string

	processors := map[string]CommandProcessor{
		"direct":   directProcessor,
		"ai":       aiProcessor,
		"rag":      ragProcessor,
		"ai-yolo":  aiYoloProcessor,
		"rag-yolo": ragYoloProcessor,
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
		// Set prompt color - use red for YOLO modes
		promptColor := "\033[1;32m" // Green
		if strings.HasSuffix(currentMode, "-yolo") {
			promptColor = "\033[1;31m" // Red for YOLO modes
		}

		fmt.Printf("%svibesh(%s)>\033[0m ", promptColor, currentMode)

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

		// Handle mode command with or without argument
		if strings.HasPrefix(input, "mode") {
			parts := strings.Fields(input)

			// Show current mode and available modes if no argument is provided
			if len(parts) == 1 {
				fmt.Printf("Current mode: %s\nAvailable modes: direct, ai, rag, ai-yolo, rag-yolo\n", currentMode)
				fmt.Print("Select mode: ")
				modeInput, err := reader.ReadString('\n')
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error reading mode:", err)
					continue
				}

				modeInput = strings.TrimSpace(modeInput)
				if _, ok := processors[modeInput]; ok {
					currentMode = modeInput

					// Add warning when switching to YOLO mode
					if strings.HasSuffix(currentMode, "-yolo") {
						fmt.Printf("\033[1;31m⚠️  CAUTION: YOLO MODE EXECUTES COMMANDS WITHOUT CONFIRMATION\033[0m\n")
					}

					fmt.Printf("Mode switched to: %s\n", currentMode)
				} else {
					fmt.Printf("Invalid mode: %s\n", modeInput)
				}
			} else if len(parts) == 2 {
				// If an argument is provided, try to switch to that mode directly
				modeInput := parts[1]
				if _, ok := processors[modeInput]; ok {
					currentMode = modeInput

					// Add warning when switching to YOLO mode
					if strings.HasSuffix(currentMode, "-yolo") {
						fmt.Printf("\033[1;31m⚠️  CAUTION: YOLO MODE EXECUTES COMMANDS WITHOUT CONFIRMATION\033[0m\n")
					}

					fmt.Printf("Mode switched to: %s\n", currentMode)
				} else {
					fmt.Printf("Invalid mode: %s\nAvailable modes: direct, ai, rag, ai-yolo, rag-yolo\n", modeInput)
				}
			} else {
				fmt.Println("Usage: mode [mode_name]")
				fmt.Println("Available modes: direct, ai, rag, ai-yolo, rag-yolo")
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
	fmt.Println("  mode [mode_name] - Switch processing mode. With no argument, it prompts for mode selection")
	fmt.Println("  history  - Display command history")
	fmt.Println("  context  - Show current directory context")
	fmt.Println("  help     - Display this help message")
	fmt.Println("\nModes:")
	fmt.Println("  direct   - Commands are executed directly in the shell")
	fmt.Println("  ai       - Natural language is converted to shell commands using AI")
	fmt.Println("  rag      - Commands are matched against a knowledge base with AI fallback")
	fmt.Println("  ai-yolo  - Like AI mode but executes commands directly without confirmation")
	fmt.Println("  rag-yolo - Like RAG mode but executes commands directly without confirmation")

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
