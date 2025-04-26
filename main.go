package main

import (
	"bufio"
	"context"
	"fmt"
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
}

func NewAIProcessor(apiKey string) *AIProcessor {
	return &AIProcessor{
		client: openai.NewClient(apiKey),
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

	// System message describing what we want
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: "You are an AI assistant helping with shell commands. Convert natural language requests into appropriate shell commands. Reply with the command only, no explanations.",
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
		Content: "Convert to a shell command: " + command,
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

	aiResponse := resp.Choices[0].Message.Content

	// Execute the AI-generated command
	shellCmd := exec.Command("sh", "-c", aiResponse)
	shellOutput, shellErr := shellCmd.CombinedOutput()

	result := fmt.Sprintf("[AI] Interpreted '%s' as:\n%s\n\nOutput:\n%s",
		command, aiResponse, string(shellOutput))

	if shellErr != nil {
		result += fmt.Sprintf("\nError: %v", shellErr)
	}

	return result, nil
}

// RAGProcessor represents a processor that uses retrieval-augmented generation
type RAGProcessor struct {
	client *openai.Client
	// Simple in-memory knowledge base for command examples
	knowledgeBase map[string]string
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
		// Create a new AI processor with directory context
		aiProcessor := AIProcessor{client: p.client}
		return aiProcessor.Process(command, history)
	}

	return "[RAG] No matching command found and AI fallback not available.", nil
}

func main() {
	fmt.Println("Vibesh - AI-Enhanced Interactive Shell")
	fmt.Println("Type 'exit' to quit, 'mode' to switch processing mode, 'help' for available commands")
	fmt.Println("Modes: 'direct' (default), 'ai', 'rag'")

	// Get OpenAI API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("Warning: OPENAI_API_KEY not set. AI and RAG modes will have limited functionality.")
	}

	reader := bufio.NewReader(os.Stdin)
	var commandHistory []string

	processors := map[string]CommandProcessor{
		"direct": &DirectShellProcessor{},
		"ai":     NewAIProcessor(apiKey),
		"rag":    NewRAGProcessor(apiKey),
	}

	currentMode := "direct"

	for {
		fmt.Printf("\033[1;32mvibesh(%s)>\033[0m ", currentMode)

		input, err := reader.ReadString('\n')
		if err != nil {
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

	if mode == "rag" {
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
