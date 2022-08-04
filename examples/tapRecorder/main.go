package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/taigrr/adb"
)

var (
	command string
	file    string

	chosen            adb.Serial
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
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
	sigChan := make(chan os.Signal)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigChan
		cancel()
	}()
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	devs, err := adb.Devices(ctx)
	if err != nil {
		fmt.Printf("Error enumerating devices: %v\n", err)
		return
	}
	devNames := []adb.Serial{}
	for _, dev := range devs {
		devNames = append(devNames, dev.SerialNo)
	}
	selected := chooseDev(devNames)

	for _, dev := range devs {
		if dev.SerialNo != selected {
			continue
		}
		if !dev.IsAuthorized {
			fmt.Printf("Dev `%s` is not authorized, authorize it to continue.\n", dev.SerialNo)
			continue
		}
		switch command {
		case "rec":
			fmt.Println("Recording taps now. Hit ctrl+c to stop.")
			t, err := dev.CaptureSequence(ctx)
			if err != nil {
				fmt.Printf("Error capturing sequence: %v\n", err)
				return
			}
			b, _ := json.Marshal(t)
			f, err := os.Create(file)
			if err != nil {
				fmt.Printf("Error creating tap file %s: %v", file, err)
				return
			}
			defer f.Close()
			f.Write(b)
		case "play":
			fmt.Println("Replaying taps now. Hit ctrl+c to stop.")
			f, err := os.Open(file)
			if err != nil {
				fmt.Printf("Error opening tap file %s: %v", file, err)
				return
			}
			defer f.Close()
			var b bytes.Buffer
			b.ReadFrom(f)
			t, err := adb.TapSequenceFromJSON(b.Bytes())
			if err != nil {
				fmt.Printf("Error parsing tap file %s: %v", file, err)
				return
			}
			dev.ReplayTapSequence(ctx, t)
		}
	}
}

func NewModel(devs []adb.Serial) Model {
	var m Model
	items := []list.Item{}
	for _, d := range devs {
		items = append(items, DevEntry(d))
	}
	m.List = list.New(items, itemDelegate{}, 0, len(devs)+15)

	return m
}

func chooseDev(devs []adb.Serial) adb.Serial {
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

	if err := tea.NewProgram(m).Start(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
	return adb.Serial(chosen)
}

type Model struct {
	List     list.Model
	quitting bool
	Choice   DevEntry
}

type DevEntry adb.Serial

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
				chosen = adb.Serial(i)
			}
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.List, cmd = m.List.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if chosen != "" {
		return quitTextStyle.Render("Chosen device: " + string(chosen))
	}
	return "\n" + m.List.View()
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
		fn = func(s string) string {
			return selectedItemStyle.Render("> " + s)
		}
	}

	fmt.Fprint(w, fn(str))
}
