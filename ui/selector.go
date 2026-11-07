package ui

import (
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	paneDBs = iota
	paneColls
)

// selectorModel is the shared two-pane database/collection checklist.
type selectorModel struct {
	dbs        []string
	colls      map[string][]string
	checked    map[string]map[string]bool
	loading    map[string]bool // per-db collection loads in flight
	loadingDBs bool
	err        string

	focus      int // paneDBs | paneColls
	dbCursor   int
	collCursor int
}

func newSelectorModel() selectorModel {
	return selectorModel{
		colls:   map[string][]string{},
		checked: map[string]map[string]bool{},
		loading: map[string]bool{},
		focus:   paneDBs,
	}
}

// selectorPage wraps the selector with page-level configuration.
type selectorPage struct {
	sel      selectorModel
	title    string
	autoLoad bool // fetch collections from the server (backup) vs preloaded (restore)
	escPage  Page
	start    func() tea.Msg // emitted by Enter on the databases pane
	hint     string
}

func newBackupSelector() selectorPage {
	return selectorPage{
		sel:      newSelectorModel(),
		title:    "Select collections to back up",
		autoLoad: true,
		escPage:  pageMode,
		start:    func() tea.Msg { return startBackupMsg{} },
		hint:     "tab: switch pane · space: toggle · enter: start · a: toggle all · esc: back",
	}
}

func newRestoreSelector(items map[string][]string) selectorPage {
	sel := newSelectorModel()
	for db, colls := range items {
		sel.colls[db] = colls
		sel.checked[db] = map[string]bool{}
		for _, c := range colls {
			sel.checked[db][c] = true // pre-checked
		}
	}
	for db := range items {
		sel.dbs = append(sel.dbs, db)
	}
	sort.Strings(sel.dbs)

	return selectorPage{
		sel:      sel,
		title:    "Select backup contents to restore",
		autoLoad: false,
		escPage:  pageRestoreLoc,
		start:    func() tea.Msg { return startRestoreMsg{} },
		hint:     "tab: switch pane · space: toggle · enter: start · a: toggle all · esc: back",
	}
}

// enter is called when the page is shown.
func (m selectorPage) enter() (selectorPage, tea.Cmd) {
	if !m.autoLoad {
		return m, nil
	}
	m.sel.loadingDBs = true
	m.sel.err = ""
	return m, loadDBsCmd()
}

func (m selectorPage) update(msg tea.Msg) (selectorPage, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		return m, nil

	case dbsLoadedMsg:
		m.sel.loadingDBs = false
		if msg.err != nil {
			m.sel.err = msg.err.Error()
			return m, nil
		}
		m.sel.setDbs(msg.names)
		if m.autoLoad && len(m.sel.dbs) > 0 {
			return m, m.sel.ensureColls(m.sel.dbs[0])
		}
		return m, nil

	case collsLoadedMsg:
		m.sel.setColls(msg)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab":
			if m.sel.focus == paneDBs {
				m.sel.focus = paneColls
			} else {
				m.sel.focus = paneDBs
			}
			return m, nil

		case "up", "k":
			return m.moveCursor(-1)
		case "down", "j":
			return m.moveCursor(1)

		case " ":
			return m.toggleFocused()

		case "enter":
			if m.sel.focus == paneDBs {
				return m, m.start
			}
			return m.toggleFocused()

		case "a":
			return m, m.toggleAll()

		case "esc":
			return m, goTo(m.escPage)
		}
	}
	return m, nil
}

// moveCursor moves the cursor in the focused pane and lazily loads
// collections when the highlighted database changes.
func (m selectorPage) moveCursor(delta int) (selectorPage, tea.Cmd) {
	if m.sel.focus == paneDBs {
		if len(m.sel.dbs) == 0 {
			return m, nil
		}
		m.sel.dbCursor = clampInt(m.sel.dbCursor+delta, 0, len(m.sel.dbs)-1)
		m.sel.collCursor = 0
		return m, m.sel.ensureColls(m.sel.dbs[m.sel.dbCursor])
	}

	db := m.highlightedDB()
	colls := m.sel.colls[db]
	if len(colls) == 0 {
		return m, nil
	}
	m.sel.collCursor = clampInt(m.sel.collCursor+delta, 0, len(colls)-1)
	return m, nil
}

// toggleFocused toggles the highlighted entry in the focused pane.
func (m selectorPage) toggleFocused() (selectorPage, tea.Cmd) {
	if m.sel.focus == paneDBs {
		db := m.highlightedDB()
		if db == "" {
			return m, nil
		}
		m.sel.toggleAll(db)
		return m, nil
	}

	db := m.highlightedDB()
	colls := m.sel.colls[db]
	if len(colls) == 0 {
		return m, nil
	}
	c := colls[clampInt(m.sel.collCursor, 0, len(colls)-1)]
	m.sel.toggle(db, c)
	return m, nil
}

// toggleAll toggles every collection of every loaded database.
func (m selectorPage) toggleAll() tea.Cmd {
	anyUnchecked := false
	for _, db := range m.sel.dbs {
		for _, c := range m.sel.colls[db] {
			if !m.sel.isChecked(db, c) {
				anyUnchecked = true
				break
			}
		}
	}
	for _, db := range m.sel.dbs {
		if m.sel.colls[db] == nil {
			continue
		}
		m.sel.checked[db] = map[string]bool{}
		for _, c := range m.sel.colls[db] {
			m.sel.checked[db][c] = anyUnchecked
		}
	}
	return nil
}

func (m selectorPage) highlightedDB() string {
	if m.sel.dbCursor < 0 || m.sel.dbCursor >= len(m.sel.dbs) {
		return ""
	}
	return m.sel.dbs[m.sel.dbCursor]
}

// workItems returns the checked db/collection pairs in display order.
func (m selectorPage) workItems() []workItem {
	var items []workItem
	for _, db := range m.sel.dbs {
		for _, c := range m.sel.colls[db] {
			if m.sel.isChecked(db, c) {
				items = append(items, workItem{db: db, coll: c})
			}
		}
	}
	return items
}

// selectedCount returns (databases, collections) currently checked.
func (m selectorPage) selectedCount() (int, int) {
	dbs, colls := 0, 0
	for _, db := range m.sel.dbs {
		n := 0
		for _, c := range m.sel.colls[db] {
			if m.sel.isChecked(db, c) {
				n++
			}
		}
		if n > 0 {
			dbs++
			colls += n
		}
	}
	return dbs, colls
}

// --- selectorModel helpers -------------------------------------------------

func (s *selectorModel) setDbs(names []string) {
	s.dbs = names
	s.loadingDBs = false
	if s.dbCursor >= len(s.dbs) {
		s.dbCursor = 0
	}
	s.collCursor = 0
}

func (s *selectorModel) setColls(msg collsLoadedMsg) {
	delete(s.loading, msg.db)
	if msg.err != nil {
		s.err = msg.err.Error()
		return
	}
	s.err = ""
	s.colls[msg.db] = msg.names
	if s.checked[msg.db] == nil {
		// Default to everything checked.
		s.checked[msg.db] = map[string]bool{}
		for _, c := range msg.names {
			s.checked[msg.db][c] = true
		}
	}
}

// ensureColls loads collections for db if they are not cached yet.
func (s selectorModel) ensureColls(db string) tea.Cmd {
	if db == "" || s.colls[db] != nil || s.loading[db] {
		return nil
	}
	s.loading[db] = true
	return loadCollsCmd(db)
}

func (s *selectorModel) isChecked(db, coll string) bool {
	return s.checked[db][coll]
}

func (s *selectorModel) toggle(db, coll string) {
	if s.checked[db] == nil {
		s.checked[db] = map[string]bool{}
	}
	s.checked[db][coll] = !s.checked[db][coll]
}

// toggleAll checks or unchecks every loaded collection of db.
func (s *selectorModel) toggleAll(db string) {
	colls := s.colls[db]
	if len(colls) == 0 {
		return
	}
	allChecked := true
	for _, c := range colls {
		if !s.checked[db][c] {
			allChecked = false
			break
		}
	}
	if s.checked[db] == nil {
		s.checked[db] = map[string]bool{}
	}
	for _, c := range colls {
		s.checked[db][c] = !allChecked
	}
}

// --- rendering -------------------------------------------------------------

func (m selectorPage) view(width, height int) string {
	maxLines := clampInt(height-16, 3, 40)
	dbPaneW := 34
	collPaneW := width - dbPaneW
	if collPaneW < 20 {
		collPaneW = 20
	}

	// Databases pane.
	var dbLines []string
	if m.sel.loadingDBs && len(m.sel.dbs) == 0 {
		dbLines = []string{infoStyle.Render("loading…")}
	} else if len(m.sel.dbs) == 0 {
		dbLines = []string{infoStyle.Render("no databases")}
	} else {
		start := clampInt(m.sel.dbCursor-maxLines/2, 0, max(0, len(m.sel.dbs)-maxLines))
		end := min(len(m.sel.dbs), start+maxLines)
		for i := start; i < end; i++ {
			db := m.sel.dbs[i]
			checkedN, totalN := 0, len(m.sel.colls[db])
			if totalN > 0 {
				for _, c := range m.sel.colls[db] {
					if m.sel.isChecked(db, c) {
						checkedN++
					}
				}
			}
			prefix := "  "
			if i == m.sel.dbCursor {
				if m.sel.focus == paneDBs {
					prefix = cursorStyle.Render("▸ ")
				} else {
					prefix = dimCursorStyle.Render("▸ ")
				}
			}
			name := db
			if totalN > 0 {
				name += infoStyle.Render("  " + itoa(checkedN) + "/" + itoa(totalN))
			} else if m.sel.loading[db] {
				name += infoStyle.Render("  …")
			}
			dbLines = append(dbLines, prefix+name)
		}
	}

	// Collections pane.
	hl := m.highlightedDB()
	collTitle := "Collections"
	if hl != "" {
		collTitle += " — " + hl
	}
	var collLines []string
	switch {
	case hl == "":
		collLines = []string{infoStyle.Render("select a database")}
	case m.sel.loading[hl]:
		collLines = []string{infoStyle.Render("loading…")}
	case len(m.sel.colls[hl]) == 0:
		collLines = []string{infoStyle.Render("no collections")}
	default:
		colls := m.sel.colls[hl]
		start := clampInt(m.sel.collCursor-maxLines/2, 0, max(0, len(colls)-maxLines))
		end := min(len(colls), start+maxLines)
		for i := start; i < end; i++ {
			c := colls[i]
			prefix := "  "
			if i == m.sel.collCursor {
				if m.sel.focus == paneColls {
					prefix = cursorStyle.Render("▸ ")
				} else {
					prefix = dimCursorStyle.Render("▸ ")
				}
			}
			box := "[ ]"
			if m.sel.isChecked(hl, c) {
				box = checkedStyle.Render("[x]")
			}
			collLines = append(collLines, prefix+box+" "+c)
		}
	}

	left := renderPane("Databases", dbLines, dbPaneW, m.sel.focus == paneDBs)
	right := renderPane(collTitle, collLines, collPaneW, m.sel.focus == paneColls)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	dbsN, collsN := m.selectedCount()

	var b strings.Builder
	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n")
	b.WriteString(infoStyle.Render("  "+itoa(dbsN)+" databases, "+itoa(collsN)+" collections selected") + "\n\n")
	b.WriteString(body)
	if m.sel.err != "" {
		b.WriteString("\n\n" + errStyle.Render("  ✘ "+m.sel.err))
	}
	b.WriteString("\n\n" + hintStyle.Render("  "+m.hint))
	return b.String()
}

// renderPane builds a bordered pane of the given outer width.
func renderPane(title string, lines []string, outerW int, focused bool) string {
	innerW := outerW - 4 // border (2) + horizontal padding (2)
	if innerW < 8 {
		innerW = 8
	}

	rendered := make([]string, 0, len(lines)+1)
	rendered = append(rendered, paneTitleStyle.Render(ansi.Truncate(title, innerW, "…")))
	for _, l := range lines {
		rendered = append(rendered, ansi.Truncate(l, innerW, "…"))
	}

	style := paneStyle
	if focused {
		style = paneFocusedStyle
	}
	return style.Render(strings.Join(rendered, "\n"))
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
