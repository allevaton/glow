package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewModelLoadsDocumentBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("# Hello\n\nSome text here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newModel(Config{Path: path}, "").(model)
	if m.state != stateShowDocument {
		t.Fatalf("expected stateShowDocument, got %s", m.state)
	}
	if !strings.Contains(m.pager.currentDocument.Body, "Some text") {
		t.Errorf("expected document body to be loaded, got %q", m.pager.currentDocument.Body)
	}
}

// documentModel returns a model parked in stateShowDocument, as if the user
// had opened a file in the pager.
func documentModel(t *testing.T) model {
	t.Helper()
	m, ok := newModel(Config{}, "# hello").(model)
	if !ok {
		t.Fatal("newModel did not return a model")
	}
	if m.state != stateShowDocument {
		t.Fatalf("setup: want stateShowDocument, got %v", m.state)
	}
	return m
}

func TestEscReturnsToFileList(t *testing.T) {
	m := documentModel(t)

	got, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	if got.(model).state != stateShowStash {
		t.Errorf("after esc: want stateShowStash, got %v", got.(model).state)
	}
}

func TestMouseBackReturnsToFileList(t *testing.T) {
	m := documentModel(t)

	got, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseBackward})

	if got.(model).state != stateShowStash {
		t.Errorf("after mouse back: want stateShowStash, got %v", got.(model).state)
	}
}

// The release half of the back button's press/release pair must not fire a
// second navigation.
func TestMouseBackReleaseIsIgnored(t *testing.T) {
	m := documentModel(t)

	got, _ := m.Update(tea.MouseReleaseMsg{Button: tea.MouseBackward})

	if got.(model).state != stateShowDocument {
		t.Errorf("after mouse back release: want stateShowDocument, got %v", got.(model).state)
	}
}

func TestMouseForwardDoesNotNavigateBack(t *testing.T) {
	m := documentModel(t)

	got, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseForward})

	if got.(model).state != stateShowDocument {
		t.Errorf("after mouse forward: want stateShowDocument, got %v", got.(model).state)
	}
}

// While a document is still loading the model sits in stateShowStash with the
// stash in stashStateLoadingDocument; mouse back must abort the load, matching
// esc.
func TestMouseBackAbortsDocumentLoad(t *testing.T) {
	m, ok := newModel(Config{Path: "."}, "").(model)
	if !ok {
		t.Fatal("newModel did not return a model")
	}
	m.stash.viewState = stashStateLoadingDocument

	got, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseBackward})

	if got.(model).stash.viewState != stashStateReady {
		t.Errorf("after mouse back during load: want stashStateReady, got %v", got.(model).stash.viewState)
	}
}
