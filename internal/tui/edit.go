package tui

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// editorCommand prepares $VISUAL or $EDITOR (default vi) on a new
// private temporary file that starts with the header as '#' lines.
func editorCommand(header string) (*exec.Cmd, string, error) {
	f, err := os.CreateTemp("", "stopan-*.md")
	if err != nil {
		return nil, "", err
	}
	var b strings.Builder
	b.WriteString("\n\n")
	for _, l := range strings.Split(header, "\n") {
		b.WriteString("# " + strings.TrimPrefix(l, "# ") + "\n")
	}
	b.WriteString("# Lines starting with '#' are dropped. Save an empty text to cancel.\n")
	_, err = f.WriteString(b.String())
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(f.Name())
		return nil, "", err
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	argv := strings.Fields(editor)
	if len(argv) == 0 {
		argv = []string{"vi"}
	}
	cmd := exec.Command(argv[0], append(argv[1:], f.Name())...)
	return cmd, f.Name(), nil
}

// readEdited returns the file's text without '#' lines and removes it.
func readEdited(path string) (string, error) {
	defer os.Remove(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var keep []string
	for _, l := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(l, "#") {
			keep = append(keep, l)
		}
	}
	return strings.TrimSpace(strings.Join(keep, "\n")), nil
}

// Edit runs the editor in the foreground, for the CLI.
func Edit(header string) (string, error) {
	cmd, path, err := editorCommand(header)
	if err != nil {
		return "", err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(path)
		return "", errors.New("editor: " + err.Error())
	}
	return readEdited(path)
}

type editedMsg struct {
	text string
	err  error
}

// editInTUI suspends the dashboard while the editor runs.
func editInTUI(header string) tea.Cmd {
	cmd, path, err := editorCommand(header)
	if err != nil {
		return func() tea.Msg { return editedMsg{err: err} }
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			os.Remove(path)
			return editedMsg{err: errors.New("editor: " + err.Error())}
		}
		s, err := readEdited(path)
		return editedMsg{text: s, err: err}
	})
}
