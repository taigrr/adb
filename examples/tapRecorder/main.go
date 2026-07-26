package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/taigrr/adb"
)

var (
	command string
	file    string

	chosen            string
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles(true).PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles(true).HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

func main() {
	flag.StringVar(&command, "command", "rec", "rec or play")
	flag.StringVar(&file, "file", "taps.json", "Name of the file to save taps to or to play from")
	flag.Parse()
	if command != "play" && command != "rec" {
		flag.PrintDefaults()
		os.Exit(1)
	}
	sigChan := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigChan
		cancel()
	}()
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	client, err := adb.New()
	if err != nil {
		fmt.Printf("Error creating adb client: %v\n", err)
		return
	}
	devs, err := client.Devices(ctx)
	if err != nil {
		fmt.Printf("Error enumerating devices: %v\n", err)
		return
	}
	devNames := []string{}
	for _, dev := range devs {
		devNames = append(devNames, dev.Serial())
	}
	selected := chooseDev(devNames)

	for _, dev := range devs {
		if dev.Serial() != selected {
			continue
		}
		if !dev.Authorized() {
			fmt.Printf("Dev `%s` is not authorized, authorize it to continue.\n", dev.Serial())
			continue
		}
		switch command {
		case "rec":
			fmt.Println("Recording taps now. Hit ctrl+c to stop.")
			seq, err := dev.Record(ctx)
			if err != nil {
				fmt.Printf("Error capturing sequence: %v\n", err)
				return
			}
			b, err := json.Marshal(seq)
			if err != nil {
				fmt.Printf("Error encoding sequence: %v\n", err)
				return
			}
			if err := os.WriteFile(file, b, 0o600); err != nil {
				fmt.Printf("Error writing tap file %s: %v\n", file, err)
				return
			}
		case "play":
			fmt.Println("Replaying taps now. Hit ctrl+c to stop.")
			b, err := os.ReadFile(file)
			if err != nil {
				fmt.Printf("Error reading tap file %s: %v\n", file, err)
				return
			}
			seq, err := adb.ParseSequence(b)
			if err != nil {
				fmt.Printf("Error parsing tap file %s: %v\n", file, err)
				return
			}
			if err := dev.Replay(ctx, seq); err != nil {
				fmt.Printf("Error replaying sequence: %v\n", err)
				return
			}
		}
	}
}

func NewModel(devs []string) Model {
	var m Model
	items := []list.Item{}
	for _, d := range devs {
		items = append(items, DevEntry(d))
	}
	m.List = list.New(items, itemDelegate{}, 0, len(devs)+15)

	return m
}

func chooseDev(devs []string) string {
	if len(devs) == 0 {
		return ""
	}
	if len(devs) == 1 {
		return devs[0]
	}
	m := NewModel(devs)
	m.List.Title = "Which device?"
	m.List.SetShowStatusBar(false)
	m.List.SetFilteringEnabled(false)
	m.List.Styles.Title = titleStyle
	m.List.Styles.PaginationStyle = paginationStyle
	m.List.Styles.HelpStyle = helpStyle

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
	return chosen
}

type Model struct {
	List     list.Model
	quitting bool
	Choice   DevEntry
}

type DevEntry string

func (d DevEntry) FilterValue() string {
	return ""
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.List.SetWidth(msg.Width)
		return m, nil
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			i, ok := m.List.SelectedItem().(DevEntry)
			if ok {
				chosen = string(i)
			}
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.List, cmd = m.List.Update(msg)
	return m, cmd
}

func (m Model) View() tea.View {
	if chosen != "" {
		return tea.NewView(quitTextStyle.Render("Chosen device: " + chosen))
	}
	return tea.NewView("\n" + m.List.View())
}

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(DevEntry)
	if !ok {
		return
	}
	str := fmt.Sprintf("%d. %s", index+1, i)
	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(strs ...string) string {
			return selectedItemStyle.Render("> " + strings.Join(strs, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}
