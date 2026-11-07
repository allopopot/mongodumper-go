package ui

import (
	"context"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// Page identifies the current screen of the wizard.
type Page int

const (
	pageURI Page = iota
	pageMode
	pageBackupSelect
	pageRestoreLoc
	pageRestoreSelect
	pageProgress
)

// Mode is the chosen operation.
type Mode int

const (
	ModeBackup Mode = iota
	ModeRestore
)

const defaultBackupDir = "./backups"

// workItem is one collection to export or import.
type workItem struct {
	db   string
	coll string
	path string // restore only
}

// Model is the root TUI model and owns all page models.
type Model struct {
	page   Page
	width  int
	height int

	uri       string
	mode      Mode
	backupDir string

	uriPage    uriModel
	modePage   modeModel
	backupSel  selectorPage
	restoreLoc restoreLocModel
	restoreSel selectorPage
	progress   progressModel
}

// New builds the initial model (page 1: URI input).
func New() Model {
	return Model{
		page:       pageURI,
		width:      80,
		height:     24,
		backupDir:  defaultBackupDir,
		uriPage:    newURIModel(),
		restoreLoc: newRestoreLocModel(),
		backupSel:  newBackupSelector(),
	}
}

func (m Model) Init() tea.Cmd {
	// The input is already focused (see newURIModel); this just starts
	// its cursor blink.
	return m.uriPage.input.Focus()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		cmd := m.forward(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			// 'q' only quits on pages without a text input.
			switch m.page {
			case pageMode, pageBackupSelect, pageRestoreSelect, pageProgress:
				return m, tea.Quit
			}
		case "esc":
			if m.page == pageURI {
				return m, tea.Quit
			}
		}
		cmd := m.forward(msg)
		return m, cmd

	case gotoPageMsg:
		m.page = msg.page
		switch msg.page {
		case pageURI:
			if m.uri != "" {
				m.uriPage.input.SetValue(m.uri)
			}
			m.uriPage.err = ""
			m.uriPage.connecting = false
			return m, m.uriPage.input.Focus()
		case pageBackupSelect:
			sel, cmd := m.backupSel.enter()
			m.backupSel = sel
			return m, cmd
		case pageRestoreLoc:
			m.restoreLoc.err = ""
			m.restoreLoc.scanning = false
			return m, m.restoreLoc.input.Focus()
		}
		return m, nil

	case connectedMsg:
		m.uriPage.connecting = false
		if msg.err != nil {
			m.uriPage.err = msg.err.Error()
			return m, nil
		}
		m.uri = m.uriPage.input.Value()
		return m, goTo(pageMode)

	case modeChosenMsg:
		m.mode = msg.mode
		if msg.mode == ModeBackup {
			return m, goTo(pageBackupSelect)
		}
		return m, goTo(pageRestoreLoc)

	case backupScanResultMsg:
		m.restoreLoc.scanning = false
		if msg.err != nil {
			m.restoreLoc.err = msg.err.Error()
			return m, nil
		}
		m.backupDir = msg.dir
		m.restoreSel = newRestoreSelector(msg.dbs)
		return m, goTo(pageRestoreSelect)

	case startBackupMsg:
		items := m.backupSel.workItems()
		if len(items) == 0 {
			return m, nil
		}
		cmd := m.startOperation(ModeBackup, items, m.backupDir, pageBackupSelect)
		return m, cmd

	case startRestoreMsg:
		items := m.restoreSel.workItems()
		for i := range items {
			items[i].path = filepath.Join(m.backupDir, items[i].db, items[i].coll+".json")
		}
		if len(items) == 0 {
			return m, nil
		}
		cmd := m.startOperation(ModeRestore, items, m.backupDir, pageRestoreSelect)
		return m, cmd
	}

	// Everything else goes to the current page.
	cmd := m.forward(msg)
	return m, cmd
}

// forward delivers msg to the active page model.
func (m *Model) forward(msg tea.Msg) tea.Cmd {
	switch m.page {
	case pageURI:
		var cmd tea.Cmd
		m.uriPage, cmd = m.uriPage.update(msg)
		return cmd
	case pageMode:
		var cmd tea.Cmd
		m.modePage, cmd = m.modePage.update(msg)
		return cmd
	case pageBackupSelect:
		var cmd tea.Cmd
		m.backupSel, cmd = m.backupSel.update(msg)
		return cmd
	case pageRestoreLoc:
		var cmd tea.Cmd
		m.restoreLoc, cmd = m.restoreLoc.update(msg)
		return cmd
	case pageRestoreSelect:
		var cmd tea.Cmd
		m.restoreSel, cmd = m.restoreSel.update(msg)
		return cmd
	case pageProgress:
		var cmd tea.Cmd
		m.progress, cmd = m.progress.update(msg)
		return cmd
	}
	return nil
}

// startOperation switches to the progress page and kicks off the runner.
func (m *Model) startOperation(mode Mode, items []workItem, outDir string, prev Page) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan tea.Msg, 256)

	m.progress = newProgressModel(mode, items, outDir, prev, cancel, ch)
	m.progress.overall.Width = max(20, m.width-14)
	m.progress.current.Width = max(20, m.width-4)
	m.page = pageProgress

	return tea.Batch(
		runOperationCmd(ctx, mode, items, outDir, ch),
		waitMsg(ch),
		m.progress.spinner.Tick,
	)
}

func (m Model) View() string {
	w := m.width - 4
	if w < 40 {
		w = 40
	}

	var content string
	switch m.page {
	case pageURI:
		content = m.uriPage.view()
	case pageMode:
		content = m.modePage.view()
	case pageBackupSelect:
		content = m.backupSel.view(w, m.height)
	case pageRestoreLoc:
		content = m.restoreLoc.view()
	case pageRestoreSelect:
		content = m.restoreSel.view(w, m.height)
	case pageProgress:
		content = m.progress.view(w, m.height)
	}
	return docStyle.Render(content)
}
