package pages

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/goiriz/kaf/internal/config"
	"github.com/goiriz/kaf/internal/kafka"
	"github.com/goiriz/kaf/internal/ui/common"
	"github.com/goiriz/kaf/internal/ui/components"
)

type setupStep int

const (
	stepMenu setupStep = iota
	stepList
	stepDeleteSelect
	stepDeleteConfirm
	stepBrokers
	stepTLS
	stepSASLMech
	stepSASLUser
	stepSASLPass
	stepSASLPassCmd
	stepSRURL
	stepProduction
	stepContextName
	stepTesting
	stepFinished
	stepCount
)

type SetupModel struct {
	step          setupStep
	cfg           *config.Config
	tempCtx       config.Context
	selectedCtx   string
	inputs        []textinput.Model
	focusedInput  int
	choices       []string
	choiceCursor  int
	spinner       spinner.Model
	testErr       error
	testSuccess   bool
	width, height int
	quitting      bool
	done          bool
}

func NewSetupModel(cfg *config.Config) SetupModel {
	s := SetupModel{
		step:   stepMenu,
		cfg:    cfg,
		inputs: make([]textinput.Model, stepCount),
	}

	for i := range s.inputs {
		ti := textinput.New()
		s.inputs[i] = ti
	}

	// Configure specific inputs
	s.inputs[stepBrokers].Placeholder = "localhost:9092"
	s.inputs[stepBrokers].Focus()

	s.inputs[stepSASLUser].Placeholder = "username"
	s.inputs[stepSASLPass].Placeholder = "password"
	s.inputs[stepSASLPass].EchoMode = textinput.EchoPassword
	s.inputs[stepSASLPass].EchoCharacter = '•'
	
	s.inputs[stepSASLPassCmd].Placeholder = "pass kafka/prod"
	s.inputs[stepSRURL].Placeholder = "http://localhost:8081"
	s.inputs[stepContextName].Placeholder = "default"

	s.spinner = spinner.New()
	s.spinner.Spinner = spinner.Dot
	s.spinner.Style = lipgloss.NewStyle().Foreground(components.GlobalTheme.PrimaryColor)

	s.choices = []string{"Add New Context", "List Contexts", "Delete Context", "Exit"}
	
	return s
}

func (m SetupModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.step == stepMenu || m.step == stepFinished {
				m.quitting = true
				return m, tea.Quit
			}
		case "enter":
			return m.nextStep()
		case "up", "k":
			if m.step == stepMenu || m.step == stepTLS || m.step == stepSASLMech || m.step == stepList || m.step == stepDeleteSelect || m.step == stepDeleteConfirm || m.step == stepProduction {
				if m.choiceCursor > 0 { m.choiceCursor-- }
			}
		case "down", "j":
			if m.step == stepMenu || m.step == stepTLS || m.step == stepSASLMech || m.step == stepList || m.step == stepDeleteSelect || m.step == stepDeleteConfirm || m.step == stepProduction {
				if m.choiceCursor < len(m.choices)-1 { m.choiceCursor++ }
			}
		case "esc":
			if m.step != stepMenu {
				m.step = stepMenu
				m.choices = []string{"Add New Context", "List Contexts", "Delete Context", "Exit"}
				m.choiceCursor = 0
				return m, nil
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case testResultMsg:
		m.step = stepFinished
		m.testErr = msg.err
		m.testSuccess = (msg.err == nil)
		if m.testSuccess {
			// Save config
			common.SaveConfig(m.cfg)
		}
		return m, nil
	}

	var cmd tea.Cmd
	if int(m.step) < len(m.inputs) && m.inputs[m.step].Focused() {
		m.inputs[m.step], cmd = m.inputs[m.step].Update(msg)
	}
	return m, cmd
}

type testResultMsg struct{ err error }

func (m *SetupModel) nextStep() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepMenu:
		switch m.choiceCursor {
		case 0: // Add
			m.step = stepBrokers
			m.inputs[stepBrokers].Focus()
			return m, nil
		case 1: // List
			m.step = stepList
			m.choices = make([]string, 0, len(m.cfg.Contexts))
			for name := range m.cfg.Contexts {
				m.choices = append(m.choices, name)
			}
			m.choiceCursor = 0
			return m, nil
		case 2: // Delete
			if len(m.cfg.Contexts) == 0 {
				return m, nil
			}
			m.step = stepDeleteSelect
			m.choices = make([]string, 0, len(m.cfg.Contexts))
			for name := range m.cfg.Contexts {
				m.choices = append(m.choices, name)
			}
			m.choiceCursor = 0
			return m, nil
		case 3: // Exit
			m.quitting = true
			return m, tea.Quit
		}
	case stepList:
		m.step = stepMenu
		m.choices = []string{"Add New Context", "List Contexts", "Delete Context", "Exit"}
		m.choiceCursor = 1
		return m, nil
	case stepDeleteSelect:
		m.selectedCtx = m.choices[m.choiceCursor]
		m.step = stepDeleteConfirm
		m.choices = []string{"Cancel", "Confirm Delete"}
		m.choiceCursor = 0
		return m, nil
	case stepDeleteConfirm:
		if m.choiceCursor == 1 {
			delete(m.cfg.Contexts, m.selectedCtx)
			if m.cfg.CurrentContext == m.selectedCtx {
				m.cfg.CurrentContext = ""
			}
			common.SaveConfig(m.cfg)
		}
		m.step = stepMenu
		m.choices = []string{"Add New Context", "List Contexts", "Delete Context", "Exit"}
		m.choiceCursor = 2
		return m, nil
	case stepBrokers:
		val := m.inputs[stepBrokers].Value()
		if val == "" { val = "localhost:9092" }
		m.tempCtx.Brokers = strings.Split(val, ",")
		m.step = stepTLS
		m.choices = []string{"No (Plain)", "Yes (TLS)"}
		m.choiceCursor = 0
		return m, nil
	case stepTLS:
		m.tempCtx.TLS = (m.choiceCursor == 1)
		m.step = stepSASLMech
		m.choices = []string{"None", "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"}
		m.choiceCursor = 0
		return m, nil
	case stepSASLMech:
		if m.choiceCursor == 0 {
			m.step = stepSRURL
			m.inputs[stepSRURL].Focus()
		} else {
			m.tempCtx.SASL = &config.SASLConfig{Mechanism: m.choices[m.choiceCursor]}
			m.step = stepSASLUser
			m.inputs[stepSASLUser].Focus()
		}
		return m, nil
	case stepSASLUser:
		m.tempCtx.SASL.Username = m.inputs[stepSASLUser].Value()
		m.step = stepSASLPass
		m.inputs[stepSASLPass].Focus()
		return m, nil
	case stepSASLPass:
		m.tempCtx.SASL.Password = m.inputs[stepSASLPass].Value()
		m.step = stepSASLPassCmd
		m.inputs[stepSASLPassCmd].Focus()
		return m, nil
	case stepSASLPassCmd:
		m.tempCtx.SASL.PasswordCmd = m.inputs[stepSASLPassCmd].Value()
		m.step = stepSRURL
		m.inputs[stepSRURL].Focus()
		return m, nil
	case stepSRURL:
		m.tempCtx.SchemaRegistryURL = m.inputs[stepSRURL].Value()
		m.step = stepProduction
		m.choices = []string{"No (Development/Test)", "Yes (Production)"}
		m.choiceCursor = 0
		return m, nil
	case stepProduction:
		m.tempCtx.IsProduction = (m.choiceCursor == 1)
		m.step = stepContextName
		m.inputs[stepContextName].Focus()
		return m, nil
	case stepContextName:
		name := m.inputs[stepContextName].Value()
		if name == "" { name = "default" }
		if m.cfg.Contexts == nil { m.cfg.Contexts = make(map[string]config.Context) }
		m.cfg.Contexts[name] = m.tempCtx
		m.cfg.CurrentContext = name
		
		m.step = stepTesting
		return m, tea.Batch(m.spinner.Tick, m.testConnection)
	case stepFinished:
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m SetupModel) testConnection() tea.Msg {
	client := kafka.NewClient(&m.tempCtx)
	defer client.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	_, err := client.ListTopics(ctx)
	return testResultMsg{err: err}
}

func (m SetupModel) View() string {
	if m.quitting {
		return "Setup cancelled.\n"
	}

	var s strings.Builder
	s.WriteString(components.TitleStyle.Render(" 🚀 KAF CONFIGURATION WIZARD ") + "\n\n")

	switch m.step {
	case stepMenu:
		s.WriteString("What would you like to do?\n\n")
		for i, choice := range m.choices {
			cursor := " "
			if m.choiceCursor == i {
				cursor = lipgloss.NewStyle().Foreground(components.GlobalTheme.PrimaryColor).Render(">")
				s.WriteString(fmt.Sprintf("%s %s\n", cursor, components.GlobalTheme.SelectedStyle.Render(choice)))
			} else {
				s.WriteString(fmt.Sprintf("%s %s\n", cursor, choice))
			}
		}

	case stepList:
		s.WriteString("Context List\n\n")
		if len(m.cfg.Contexts) == 0 {
			s.WriteString(components.GlobalTheme.DimStyle.Render(" No contexts configured.") + "\n")
		} else {
			for name, ctx := range m.cfg.Contexts {
				active := ""
				if name == m.cfg.CurrentContext {
					active = components.GlobalTheme.InfoStyle.Render(" (active)")
				}
				s.WriteString(fmt.Sprintf(" • %s: %v%s\n", name, ctx.Brokers, active))
			}
		}
		s.WriteString("\n" + components.GlobalTheme.DimStyle.Render("Press ENTER to return to menu"))

	case stepDeleteSelect:
		s.WriteString("Delete Context\n")
		s.WriteString("Select a context to remove:\n\n")
		for i, choice := range m.choices {
			if m.choiceCursor == i {
				s.WriteString(lipgloss.NewStyle().Foreground(components.GlobalTheme.PrimaryColor).Render("> ") + components.GlobalTheme.SelectedStyle.Render(choice) + "\n")
			} else {
				s.WriteString("  " + choice + "\n")
			}
		}

	case stepDeleteConfirm:
		s.WriteString("Confirm Deletion\n")
		s.WriteString(fmt.Sprintf("Are you sure you want to delete context '%s'?\n\n", m.selectedCtx))
		for i, choice := range m.choices {
			if m.choiceCursor == i {
				style := components.GlobalTheme.SelectedStyle
				if choice == "Confirm Delete" {
					style = components.GlobalTheme.ToastStyle
				}
				s.WriteString(lipgloss.NewStyle().Foreground(components.GlobalTheme.PrimaryColor).Render("> ") + style.Render(choice) + "\n")
			} else {
				s.WriteString("  " + choice + "\n")
			}
		}

	case stepBrokers:
		s.WriteString("Step 1: Kafka Brokers\n")
		s.WriteString(components.GlobalTheme.DimStyle.Render("Enter comma-separated addresses (e.g. localhost:9092,kafka:9093)") + "\n\n")
		s.WriteString(m.inputs[stepBrokers].View())
	
	case stepTLS:
		s.WriteString("Step 2: Security (TLS)\n")
		s.WriteString("Does this cluster require a TLS connection?\n\n")
		for i, choice := range m.choices {
			if m.choiceCursor == i {
				s.WriteString(lipgloss.NewStyle().Foreground(components.GlobalTheme.PrimaryColor).Render("> ") + components.GlobalTheme.SelectedStyle.Render(choice) + "\n")
			} else {
				s.WriteString("  " + choice + "\n")
			}
		}

	case stepSASLMech:
		s.WriteString("Step 3: Authentication (SASL)\n")
		s.WriteString("Select SASL mechanism:\n\n")
		for i, choice := range m.choices {
			if m.choiceCursor == i {
				s.WriteString(lipgloss.NewStyle().Foreground(components.GlobalTheme.PrimaryColor).Render("> ") + components.GlobalTheme.SelectedStyle.Render(choice) + "\n")
			} else {
				s.WriteString("  " + choice + "\n")
			}
		}

	case stepSASLUser:
		s.WriteString("SASL Credentials\n")
		s.WriteString("Username:\n\n")
		s.WriteString(m.inputs[stepSASLUser].View())
	
	case stepSASLPass:
		s.WriteString("SASL Credentials\n")
		s.WriteString("Password (leave empty if using Password Command):\n\n")
		s.WriteString(m.inputs[stepSASLPass].View())
		s.WriteString("\n\n" + components.GlobalTheme.ToastStyle.Render(" WARNING: Plain-text passwords are saved in ~/.kaf.yaml "))

	case stepSASLPassCmd:
		s.WriteString("Secure Password Retrieval\n")
		s.WriteString("Command to fetch password (optional, e.g. 'pass kafka/prod'):\n\n")
		s.WriteString(m.inputs[stepSASLPassCmd].View())

	case stepSRURL:
		s.WriteString("Step 4: Schema Registry\n")
		s.WriteString("URL (optional, for Avro support):\n\n")
		s.WriteString(m.inputs[stepSRURL].View())

	case stepProduction:
		s.WriteString("Step 5: Production Safeguards\n")
		s.WriteString("Is this a PRODUCTION cluster? (Enables strict safety interlocks)\n\n")
		for i, choice := range m.choices {
			if m.choiceCursor == i {
				style := components.GlobalTheme.SelectedStyle
				if i == 1 { style = components.GlobalTheme.ToastStyle }
				s.WriteString(lipgloss.NewStyle().Foreground(components.GlobalTheme.PrimaryColor).Render("> ") + style.Render(choice) + "\n")
			} else {
				s.WriteString("  " + choice + "\n")
			}
		}

	case stepContextName:
		s.WriteString("Final Step: Context Name\n")
		s.WriteString("How should we name this cluster configuration?\n\n")
		s.WriteString(m.inputs[stepContextName].View())

	case stepTesting:
		s.WriteString(fmt.Sprintf("\n %s Testing connection to brokers...", m.spinner.View()))

	case stepFinished:
		if m.testSuccess {
			s.WriteString(components.GlobalTheme.InfoStyle.Render(" ✅ Success! ") + "\n\n")
			s.WriteString(fmt.Sprintf("Context '%s' has been successfully configured and tested.\n", m.cfg.CurrentContext))
		} else {
			s.WriteString(components.GlobalTheme.ToastStyle.Render(" ❌ Connection Failed ") + "\n\n")
			s.WriteString(fmt.Sprintf("Error: %v\n\n", m.testErr))
			s.WriteString("The context was saved anyway, but you might need to check your settings.")
		}
		s.WriteString("\n\nPress ENTER to finish.")
	}

	if m.step != stepMenu && m.step != stepFinished && m.step != stepTesting {
		s.WriteString("\n\n" + components.GlobalTheme.DimStyle.Render("press ENTER to continue • ESC to cancel"))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(components.GlobalTheme.PrimaryColor).
		Padding(1, 2).
		Margin(1, 2).
		Render(s.String())
}
