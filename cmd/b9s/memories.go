package main

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanderheijden86/b9s/internal/datasource"
)

// runMemories renders the graph preview through bd's read-only records command.
func runMemories(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("memories", flag.ContinueOnError)
	flags.SetOutput(stderr)
	project := flags.String("project", ".", "graph preview project directory")
	id := flags.String("id", "", "show one Memory by local or canonical ID")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "Usage: b9s memories [--project DIR] [--id ID]")
		return 2
	}
	graph, err := datasource.LoadGraphPreview(*project)
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
			if bead.ID == selected || graphLocalID(bead.ID) == selected || strings.TrimPrefix(graphLocalID(bead.ID), "beads/") == selected {
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
