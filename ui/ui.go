// Package ui provides the main UI for the glow application.
package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/glow/v2/utils"
	"github.com/charmbracelet/log"
	"github.com/muesli/gitcha"
	te "github.com/muesli/termenv"
)

const (
	statusMessageTimeout = time.Second * 3 // how long to show status messages like "stashed!"
	ellipsis             = "…"
)

var (
	config Config

	markdownExtensions = []string{
		"*.md", "*.mdown", "*.mkdn", "*.mkd", "*.markdown",
	}
)

// NewProgram returns a new Tea program.
func NewProgram(cfg Config, content string) *tea.Program {
	log.Debug(
		"Starting glow",
		"high_perf_pager",
		cfg.HighPerformancePager,
		"glamour",
		cfg.GlamourEnabled,
	)

	config = cfg
	// Mouse support is always on in TUI mode so the file list can be
	// driven by clicks and the scroll wheel. The legacy --mouse flag /
	// `mouse:` config option is now a no-op.
	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
	_ = cfg.EnableMouse
	m := newModel(cfg, content)
	return tea.NewProgram(m, opts...)
}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

type (
	initLocalFileSearchMsg struct {
		cwd string
		ch  chan gitcha.SearchResult
	}
)

type (
	foundLocalFileMsg       gitcha.SearchResult
	localFileSearchFinished struct{}
	statusMessageTimeoutMsg applicationContext
)

type (
	// rescanTickMsg is emitted by scheduleRescan when it's time to consider
	// running another rescan.
	rescanTickMsg struct{}

	// rescanFinishedMsg carries the result of a periodic rescan: the full
	// set of file paths currently visible to gitcha.
	rescanFinishedMsg struct {
		paths []string
	}
)

// applicationContext indicates the area of the application something applies
// to. Occasionally used as an argument to commands and messages.
type applicationContext int

const (
	stashContext applicationContext = iota
	pagerContext
)

// state is the top-level application state.
type state int

const (
	stateShowStash state = iota
	stateShowDocument
)

func (s state) String() string {
	return map[state]string{
		stateShowStash:    "showing file listing",
		stateShowDocument: "showing document",
	}[s]
}

// Common stuff we'll need to access in all models.
type commonModel struct {
	cfg    Config
	cwd    string
	width  int
	height int
}

type model struct {
	common   *commonModel
	state    state
	fatalErr error

	// Sub-models
	stash stashModel
	pager pagerModel

	// Channel that receives paths to local markdown files
	// (via the github.com/muesli/gitcha package)
	localFileFinder chan gitcha.SearchResult

	// autoRefreshStarted is set to true after the first auto-refresh tick
	// chain is scheduled. This prevents leaking additional concurrent tick
	// chains when the user presses F (manual refresh), which resets
	// m.stash.loaded and goes through the same localFileSearchFinished path.
	autoRefreshStarted bool
}

// unloadDocument unloads a document from the pager. Note that while this
// method alters the model we also need to send along any commands returned.
func (m *model) unloadDocument() []tea.Cmd {
	m.state = stateShowStash
	m.stash.viewState = stashStateReady
	m.pager.unload()
	m.pager.showHelp = false

	var batch []tea.Cmd
	if m.pager.viewport.HighPerformanceRendering {
		batch = append(batch, tea.ClearScrollArea) //nolint:staticcheck
	}

	if !m.stash.shouldSpin() {
		batch = append(batch, m.stash.spinner.Tick)
	}
	return batch
}

func newModel(cfg Config, content string) tea.Model {
	initSections()

	if cfg.GlamourStyle == styles.AutoStyle {
		if te.HasDarkBackground() {
			cfg.GlamourStyle = styles.DarkStyle
		} else {
			cfg.GlamourStyle = styles.LightStyle
		}
	}

	common := commonModel{
		cfg: cfg,
	}

	m := model{
		common: &common,
		state:  stateShowStash,
		pager:  newPagerModel(&common),
		stash:  newStashModel(&common),
	}

	path := cfg.Path
	if path == "" && content != "" {
		m.state = stateShowDocument
		m.pager.currentDocument = markdown{Body: content}
		return m
	}

	if path == "" {
		path = "."
	}
	info, err := os.Stat(path)
	if err != nil {
		log.Error("unable to stat file", "file", path, "error", err)
		m.fatalErr = err
		return m
	}
	if info.IsDir() {
		m.state = stateShowStash
	} else {
		cwd, _ := os.Getwd()
		m.state = stateShowDocument
		m.pager.currentDocument = markdown{
			localPath: path,
			Note:      stripAbsolutePath(path, cwd),
			Modtime:   info.ModTime(),
		}
	}

	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.stash.spinner.Tick}

	switch m.state {
	case stateShowStash:
		cmds = append(cmds, findLocalFiles(*m.common))
	case stateShowDocument:
		content, err := os.ReadFile(m.common.cfg.Path)
		if err != nil {
			log.Error("unable to read file", "file", m.common.cfg.Path, "error", err)
			return func() tea.Msg { return errMsg{err} }
		}
		body := string(utils.RemoveFrontmatter(content))
		cmds = append(cmds, renderWithGlamour(m.pager, body))
	}

	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If there's been an error, any key exits
	if m.fatalErr != nil {
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.state == stateShowDocument || m.stash.viewState == stashStateLoadingDocument {
				batch := m.unloadDocument()
				return m, tea.Batch(batch...)
			}
		case "r":
			var cmd tea.Cmd
			if m.state == stateShowStash {
				// pass through all keys if we're editing the filter
				if m.stash.filterState == filtering {
					m.stash, cmd = m.stash.update(msg)
					return m, cmd
				}
				m.stash.markdowns = nil
				return m, m.Init()
			}

		case "q":
			var cmd tea.Cmd

			switch m.state { //nolint:exhaustive
			case stateShowStash:
				// pass through all keys if we're editing the filter
				if m.stash.filterState == filtering {
					m.stash, cmd = m.stash.update(msg)
					return m, cmd
				}
			}

			return m, tea.Quit

		case "left", "h", "delete":
			if m.state == stateShowDocument {
				cmds = append(cmds, m.unloadDocument()...)
				return m, tea.Batch(cmds...)
			}

		case "ctrl+z":
			return m, tea.Suspend

		// Ctrl+C always quits no matter where in the application you are.
		case "ctrl+c":
			return m, tea.Quit
		}

	// Window size is received when starting up and on every resize
	case tea.WindowSizeMsg:
		m.common.width = msg.Width
		m.common.height = msg.Height
		m.stash.setSize(msg.Width, msg.Height)
		m.pager.setSize(msg.Width, msg.Height)

	case initLocalFileSearchMsg:
		m.localFileFinder = msg.ch
		m.common.cwd = msg.cwd
		cmds = append(cmds, findNextLocalFile(m))

	case fetchedMarkdownMsg:
		// We've loaded a markdown file's contents for rendering
		m.pager.currentDocument = *msg
		body := string(utils.RemoveFrontmatter([]byte(msg.Body)))
		cmds = append(cmds, renderWithGlamour(m.pager, body))

	case contentRenderedMsg:
		m.state = stateShowDocument

	case localFileSearchFinished:
		// Always pass these messages to the stash so we can keep it updated
		// about network activity, even if the user isn't currently viewing
		// the stash.
		stashModel, cmd := m.stash.update(msg)
		m.stash = stashModel
		cmds = append(cmds, cmd)
		// Schedule the auto-refresh tick chain exactly once. We cannot gate
		// on m.stash.loaded because the F key resets loaded=false before
		// re-running findLocalFiles, so every F press would also arrive here
		// with loaded=false and schedule a second chain.
		if !m.autoRefreshStarted && m.common.cfg.RefreshInterval > 0 {
			m.autoRefreshStarted = true
			cmds = append(cmds, scheduleRescan(m.common.cfg.RefreshInterval))
		}
		return m, tea.Batch(cmds...)

	case foundLocalFileMsg:
		newMd := localFileToMarkdown(m.common.cwd, gitcha.SearchResult(msg))
		m.stash.addMarkdowns(newMd)
		if m.stash.filterApplied() {
			newMd.buildFilterValue()
		}
		if m.stash.shouldUpdateFilter() {
			cmds = append(cmds, filterMarkdowns(m.stash))
		}
		cmds = append(cmds, findNextLocalFile(m))

	case filteredMarkdownMsg:
		if m.state == stateShowDocument {
			newStashModel, cmd := m.stash.update(msg)
			m.stash = newStashModel
			cmds = append(cmds, cmd)
		}

	case rescanTickMsg:
		if m.common.cfg.RefreshInterval <= 0 {
			break
		}
		// Skip this tick if we shouldn't rescan right now; re-arm.
		if m.state != stateShowStash {
			cmds = append(cmds, scheduleRescan(m.common.cfg.RefreshInterval))
			break
		}
		if m.stash.rescanInFlight {
			cmds = append(cmds, scheduleRescan(m.common.cfg.RefreshInterval))
			break
		}
		if m.stash.viewState != stashStateReady {
			cmds = append(cmds, scheduleRescan(m.common.cfg.RefreshInterval))
			break
		}
		if m.stash.filterState == filtering {
			cmds = append(cmds, scheduleRescan(m.common.cfg.RefreshInterval))
			break
		}
		m.stash.rescanInFlight = true
		cmds = append(cmds, rescanLocalFiles(*m.common))

	case rescanFinishedMsg:
		m.stash.rescanInFlight = false
		cwd := m.common.cwd
		m.stash.applyRescan(msg.paths, func(p string) *markdown {
			return pathToMarkdown(cwd, p)
		})
		if m.stash.filterApplied() {
			cmds = append(cmds, filterMarkdowns(m.stash))
		}
		if m.common.cfg.RefreshInterval > 0 {
			cmds = append(cmds, scheduleRescan(m.common.cfg.RefreshInterval))
		}
	}

	// Process children
	switch m.state {
	case stateShowStash:
		newStashModel, cmd := m.stash.update(msg)
		m.stash = newStashModel
		cmds = append(cmds, cmd)

	case stateShowDocument:
		newPagerModel, cmd := m.pager.update(msg)
		m.pager = newPagerModel
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.fatalErr != nil {
		return errorView(m.fatalErr, true)
	}

	switch m.state { //nolint:exhaustive
	case stateShowDocument:
		return m.pager.View()
	default:
		return m.stash.view()
	}
}

func errorView(err error, fatal bool) string {
	exitMsg := "press any key to "
	if fatal {
		exitMsg += "exit"
	} else {
		exitMsg += "return"
	}
	s := fmt.Sprintf("%s\n\n%v\n\n%s",
		errorTitleStyle.Render("ERROR"),
		err,
		subtleStyle.Render(exitMsg),
	)
	return "\n" + indent(s, 3)
}

// COMMANDS

func findLocalFiles(m commonModel) tea.Cmd {
	return func() tea.Msg {
		log.Info("findLocalFiles")
		var (
			cwd = m.cfg.Path
			err error
		)

		if cwd == "" {
			cwd, err = os.Getwd()
		} else {
			var info os.FileInfo
			info, err = os.Stat(cwd)
			if err == nil && info.IsDir() {
				cwd, err = filepath.Abs(cwd)
			}
		}

		// Note that this is one error check for both cases above
		if err != nil {
			log.Error("error finding local files", "error", err)
			return errMsg{err}
		}

		log.Debug("local directory is", "cwd", cwd)

		// Switch between FindFiles and FindAllFiles to bypass .gitignore rules
		var ch chan gitcha.SearchResult
		if m.cfg.ShowAllFiles {
			ch, err = gitcha.FindAllFilesExcept(cwd, markdownExtensions, nil)
		} else {
			ch, err = gitcha.FindFilesExcept(cwd, markdownExtensions, ignorePatterns(m))
		}

		if err != nil {
			log.Error("error finding local files", "error", err)
			return errMsg{err}
		}

		return initLocalFileSearchMsg{ch: ch, cwd: cwd}
	}
}

func findNextLocalFile(m model) tea.Cmd {
	return func() tea.Msg {
		res, ok := <-m.localFileFinder

		if ok {
			// Okay now find the next one
			return foundLocalFileMsg(res)
		}
		// We're done
		log.Debug("local file search finished")
		return localFileSearchFinished{}
	}
}

// rescanLocalFiles runs the same gitcha walk as findLocalFiles but drains
// the result channel synchronously into a single rescanFinishedMsg so the
// stash can apply the diff atomically.
func rescanLocalFiles(m commonModel) tea.Cmd {
	return func() tea.Msg {
		var (
			cwd = m.cfg.Path
			err error
		)

		if cwd == "" {
			cwd, err = os.Getwd()
		} else {
			var info os.FileInfo
			info, err = os.Stat(cwd)
			if err == nil && info.IsDir() {
				cwd, err = filepath.Abs(cwd)
			}
		}
		if err != nil {
			log.Debug("rescanLocalFiles: cwd resolve failed", "error", err)
			return rescanFinishedMsg{paths: nil}
		}

		var ch chan gitcha.SearchResult
		if m.cfg.ShowAllFiles {
			ch, err = gitcha.FindAllFilesExcept(cwd, markdownExtensions, nil)
		} else {
			ch, err = gitcha.FindFilesExcept(cwd, markdownExtensions, ignorePatterns(m))
		}
		if err != nil {
			log.Debug("rescanLocalFiles: gitcha failed", "error", err)
			return rescanFinishedMsg{paths: nil}
		}

		var paths []string
		for res := range ch {
			paths = append(paths, res.Path)
		}
		return rescanFinishedMsg{paths: paths}
	}
}

// scheduleRescan returns a command that fires a rescanTickMsg after d.
// Callers should not call this with d <= 0; check the interval first.
func scheduleRescan(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return rescanTickMsg{}
	})
}

func waitForStatusMessageTimeout(appCtx applicationContext, t *time.Timer) tea.Cmd {
	return func() tea.Msg {
		<-t.C
		return statusMessageTimeoutMsg(appCtx)
	}
}

// ETC

// Convert a Gitcha result to an internal representation of a markdown
// document. Note that we could be doing things like checking if the file is
// a directory, but we trust that gitcha has already done that.
func localFileToMarkdown(cwd string, res gitcha.SearchResult) *markdown {
	return &markdown{
		localPath: res.Path,
		Note:      stripAbsolutePath(res.Path, cwd),
		Modtime:   res.Info.ModTime(),
	}
}

// pathToMarkdown builds a *markdown for a path discovered by a rescan,
// statting the file for its modtime. Returns nil if the file can no longer
// be stat'd (it may have been deleted between the walk and now).
func pathToMarkdown(cwd, path string) *markdown {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	return &markdown{
		localPath: path,
		Note:      stripAbsolutePath(path, cwd),
		Modtime:   info.ModTime(),
	}
}

func stripAbsolutePath(fullPath, cwd string) string {
	fp, _ := filepath.EvalSymlinks(fullPath)
	cp, _ := filepath.EvalSymlinks(cwd)
	return strings.ReplaceAll(fp, cp+string(os.PathSeparator), "")
}

// Lightweight version of reflow's indent function.
func indent(s string, n int) string {
	if n <= 0 || s == "" {
		return s
	}
	l := strings.Split(s, "\n")
	b := strings.Builder{}
	i := strings.Repeat(" ", n)
	for _, v := range l {
		fmt.Fprintf(&b, "%s%s\n", i, v)
	}
	return b.String()
}
