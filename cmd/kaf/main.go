package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui"
	"github.com/goiriz/kaf/internal/config"
	"github.com/goiriz/kaf/internal/ui/common"
	"github.com/goiriz/kaf/internal/ui/pages"
	"github.com/spf13/pflag"
)

var Version = "1.0.0"

func runSetupWizard(cfg *config.Config) config.Context {
	m := pages.NewSetupModel(cfg)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running setup wizard: %v\n", err)
		os.Exit(1)
	}
	
	// Reload config to get the newly saved context
	newCfg, _ := common.LoadConfig()
	*cfg = *newCfg
	
	if cfg.CurrentContext != "" {
		return cfg.Contexts[cfg.CurrentContext]
	}
	var Version = "dev"

	func getVersion() string {
		if Version != "dev" {
			return Version
		}

		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}

		return Version
	}

	func main() {
		broker := pflag.StringP("broker", "b", "", "Kafka broker address(es), comma-separated")
		contextName := pflag.StringP("context", "c", "", "Use a specific context from config")
		enableWrite := pflag.Bool("write", false, "Enable write mode (allows destructive actions)")
		isProduction := pflag.BoolP("production", "p", false, "Force production safety interlocks")
		version := pflag.Bool("version", false, "Show version information")
		help := pflag.BoolP("help", "h", false, "Show help message")
		pflag.Parse()

		if *version {
			fmt.Printf("kaf %s\n", getVersion())
			return
		}

	cfg, _ := common.LoadConfig()
	common.InitLogger(cfg.LogPath, cfg.LogMaxSizeMB)
	defer common.CloseLogger()

	if *help {
		showHelp()
		return
	}

	// Handle 'setup' command
	if pflag.NArg() > 0 && pflag.Arg(0) == "setup" {
		runSetupWizard(cfg)
		return
	}

	activeContextName := *contextName
	if activeContextName == "" {
		activeContextName = cfg.CurrentContext
	}

	var activeContext config.Context
	if activeContextName != "" {
		if ctx, ok := cfg.Contexts[activeContextName]; ok {
			activeContext = ctx
		} else if *contextName != "" {
			fmt.Printf("Context '%s' not found in config\n", activeContextName)
			os.Exit(1)
		}
	}

	// Override with flag or env if provided
	if *broker != "" {
		activeContext.Brokers = strings.Split(*broker, ",")
	} else if envBrokers := os.Getenv("KAFKA_BROKERS"); envBrokers != "" && len(activeContext.Brokers) == 0 {
		activeContext.Brokers = strings.Split(envBrokers, ",")
	}
	
	if *enableWrite {
		activeContext.WriteEnabled = true
	}

	if *isProduction {
		activeContext.IsProduction = true
	}

	if len(activeContext.Brokers) == 0 {
		activeContext = runSetupWizard(cfg)
	}

	client := kafka.NewClient(&activeContext)
	defer client.Close()
	
	model := ui.NewModel(cfg, client)

	p := tea.NewProgram(&model, tea.WithAltScreen())
	
	defer func() {
		if r := recover(); r != nil {
			common.Log("CRASH: %v\n%s", r, string(debug.Stack()))
			p.Quit()
			fmt.Printf("KAF crashed! See kaf.log for details. Error: %v\n", r)
			os.Exit(1)
		}
	}()

	common.Log("Starting KAF on brokers %v", activeContext.Brokers)
	if _, err := p.Run(); err != nil {
		common.Log("Fatal error: %v", err)
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

func showHelp() {
	fmt.Println("kaf - a simple kafka client")
	fmt.Println("\nUsage:")
	fmt.Println("  kaf setup                 Run the configuration wizard")
	fmt.Println("  kaf --broker <address>    Connect to a Kafka broker")
	fmt.Println("  kaf --context <name>      Use a saved context from ~/.kaf.yaml")
	fmt.Println("  kaf --write               Enable write mode (dangerous)")
	fmt.Println("  kaf --production, -p      Force production safety interlocks")
	fmt.Println("\nConfiguration (~/.kaf.yaml):")
	fmt.Println("  contexts:")
	fmt.Println("    local:")
	fmt.Println("      brokers: [\"localhost:9092\"]")
	fmt.Println("    prod:")
	fmt.Println("      brokers: [\"prod-1:9092\", \"prod-2:9092\"]")
	fmt.Println("      tls: true")
}
