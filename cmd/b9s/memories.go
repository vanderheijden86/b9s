package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/internal/bdrun"
	"github.com/vanderheijden86/b9s/internal/datasource"
	"github.com/vanderheijden86/b9s/pkg/config"
	"github.com/vanderheijden86/b9s/pkg/ui"
	"golang.org/x/term"
)

// runMemories browses the graph preview's Memories in a TUI on a terminal and
// prints them otherwise. Every read goes through the preview bd (ADR 0037).
func runMemories(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("memories", flag.ContinueOnError)
	flags.SetOutput(stderr)
	project := flags.String("project", ".", "graph preview project directory")
	id := flags.String("id", "", "print one Memory by local or canonical ID")
	printOnly := flags.Bool("print", false, "print Memories instead of opening the browser")
	reprobe := flags.Bool("reprobe", false, "ask bd again whether it supports Memory Beads instead of using the cached answer")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "Usage: b9s memories [--project DIR] [--print] [--id ID] [--reprobe]")
		return 2
	}
	if cfg, err := config.Load(); err == nil {
		applyMemoryLever(cfg)
	}
	if *reprobe {
		forgetMemoryProbe(stderr)
	}
	if capability := datasource.DetectMemory(context.Background(), *project); capability.Err != nil {
		reason, ok := datasource.MemoryUnavailableReason(capability.Err)
		if !ok {
			reason = capability.Err.Error()
		}
		fmt.Fprintf(stderr, "Memories unavailable: %s (%s)\n", reason, capability.State)
		return 2
	}
	client, err := datasource.OpenGraphPreview(*project)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if !*printOnly && *id == "" && isTerminal(stdout) && term.IsTerminal(int(os.Stdin.Fd())) {
		if err := runMemoryBrowser(client, *project); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	}
	graph, err := loadMemoryGraph(client, *id)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	view, err := renderMemories(graph, *id)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, view)
	return 0
}

// applyMemoryLever carries the config key into the environment, where
// DetectMemory reads it. It only ever turns Memory off.
func applyMemoryLever(cfg config.Config) {
	if cfg.Memory == "off" {
		_ = os.Setenv(datasource.MemoryLeverEnv, "off")
	}
}

// forgetMemoryProbe drops the cached Memory probe for the bd on PATH, for a
// bd replaced in place without its size or modification time changing.
func forgetMemoryProbe(stderr io.Writer) {
	bd, ok := bdrun.Resolve()
	if !ok {
		return
	}
	if err := bdrun.ForgetProbe(bd, bdrun.DefaultProbeCacheFile()); err != nil {
		fmt.Fprintf(stderr, "Warning: clearing the bd capability cache: %v\n", err)
	}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// loadMemoryGraph reads the inventory, then the incident Links of the
// Memories that will be printed. Incoming Links are owned by other Beads, so
// only bd links reports them per Memory.
func loadMemoryGraph(client *datasource.GraphPreviewClient, selected string) (datasource.GraphPreview, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	graph, err := client.Inventory(ctx)
	if err != nil {
		return datasource.GraphPreview{}, err
	}
	links := make(map[string]datasource.GraphLink)
	for _, link := range graph.Links {
		links[link.ID] = link
	}
	for _, bead := range graph.Beads {
		if bead.Kind != "memory" || (selected != "" && !memoryMatches(bead, selected)) {
			continue
		}
		incident, err := client.Links(ctx, bead.ID)
		if err != nil {
			return datasource.GraphPreview{}, err
		}
		for _, link := range incident {
			links[link.ID] = link
		}
	}
	graph.Links = graph.Links[:0]
	for _, link := range links {
		graph.Links = append(graph.Links, link)
	}
	sort.Slice(graph.Links, func(i, j int) bool { return graph.Links[i].ID < graph.Links[j].ID })
	return graph, nil
}

func memoryMatches(bead datasource.GraphBead, selected string) bool {
	return bead.ID == selected || graphLocalID(bead.ID) == selected || strings.TrimPrefix(graphLocalID(bead.ID), "beads/") == selected
}

// memoryProgram adapts the browser's typed Update to tea.Model.
type memoryProgram struct{ ui.MemoryBrowser }

func (p memoryProgram) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	browser, cmd := p.MemoryBrowser.Update(msg)
	return memoryProgram{browser}, cmd
}

func runMemoryBrowser(client *datasource.GraphPreviewClient, project string) error {
	name := filepath.Base(project)
	if abs, err := filepath.Abs(project); err == nil {
		name = filepath.Base(abs)
	}
	program := tea.NewProgram(memoryProgram{ui.NewMemoryBrowser(client, name)}, tea.WithAltScreen())
	if v := os.Getenv("B9S_TUI_AUTOCLOSE_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			timer := time.AfterFunc(time.Duration(ms)*time.Millisecond, program.Quit)
			defer timer.Stop()
		}
	}
	_, err := program.Run()
	return err
}

func renderMemories(graph datasource.GraphPreview, selected string) (string, error) {
	byID := make(map[string]datasource.GraphBead, len(graph.Beads))
	memories := make([]datasource.GraphBead, 0)
	for _, bead := range graph.Beads {
		byID[bead.ID] = bead
		if bead.Kind == "memory" {
			memories = append(memories, bead)
		}
	}
	sort.Slice(memories, func(i, j int) bool { return memories[i].ID < memories[j].ID })
	if selected != "" {
		matches := memories[:0]
		for _, bead := range memories {
			if memoryMatches(bead, selected) {
				matches = append(matches, bead)
			}
		}
		if len(matches) != 1 {
			return "", fmt.Errorf("Memory %q was not found or is ambiguous", selected)
		}
		memories = matches
	}
	heading := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	var lines []string
	lines = append(lines, heading.Render(fmt.Sprintf("Memory Beads (%d)", len(memories))))
	if len(memories) == 0 {
		return strings.Join(append(lines, muted.Render("No Memories in this graph preview.")), "\n"), nil
	}
	for _, bead := range memories {
		lines = append(lines, "", heading.Render(cleanGraphText(bead.Properties.Title))+"  "+muted.Render(cleanGraphText(graphLocalID(bead.ID))))
		lines = append(lines, muted.Render("version "+cleanGraphText(bead.Version)))
		if bead.Properties.Body != "" {
			lines = append(lines, cleanGraphText(bead.Properties.Body))
		}
		for _, link := range graph.Links {
			if link.Source == bead.ID {
				lines = append(lines, renderMemoryLink("→", link, link.Target, byID))
			} else if link.Target == bead.ID {
				lines = append(lines, renderMemoryLink("←", link, link.Source, byID))
			}
		}
	}
	return strings.Join(lines, "\n"), nil
}

func renderMemoryLink(arrow string, link datasource.GraphLink, otherID string, byID map[string]datasource.GraphBead) string {
	other := byID[otherID]
	name := graphLocalID(otherID)
	if other.Properties.Title != "" {
		name += " (" + other.Properties.Title + ")"
	}
	line := fmt.Sprintf("  %s %s %s", arrow, graphLocalID(link.Type), name)
	if link.Properties.Note != "" {
		line += "  " + link.Properties.Note
	}
	return cleanGraphText(line)
}

func graphLocalID(id string) string {
	for _, marker := range []string{"/beads/", "/links/", "/types/"} {
		if index := strings.LastIndex(id, marker); index >= 0 {
			return strings.TrimPrefix(marker, "/") + id[index+len(marker):]
		}
	}
	return id
}

func cleanGraphText(input string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, input)
}
