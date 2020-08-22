package gui

import (
	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazygit/pkg/gui/presentation"
)

// TODO: if we don't end up using this, delete it
const (
	CONTAINS_NOTHING = iota
	CONTAINS_COMMITS
	CONTAINS_FILES
	CONTAINS_BRANCHES
)

type ListContext struct {
	ViewName            string
	ContextKey          string
	GetItemsLength      func() int
	GetDisplayStrings   func() [][]string
	OnFocus             func() error
	OnFocusLost         func() error
	OnClickSelectedItem func() error
	SelectedItem        func() ListItem
	GetPanelState       func() IListPanelState

	Gui               *Gui
	RendersToMainView bool
	Kind              int
	ParentContext     Context
	WindowName        string
	Contains          int
}

type ListItem interface {
	// ID is a SHA when the item is a commit, a filename when the item is a file, 'stash@{4}' when it's a stash entry, 'my_branch' when it's a branch
	ID() string

	// Description is something we would show in a message e.g. '123as14: push blah' for a commit
	Description() string
}

func (lc *ListContext) GetSelectedItem() ListItem {
	return lc.SelectedItem()
}

func (lc *ListContext) GetContains() int {
	return lc.Contains
}

func (lc *ListContext) SetWindowName(windowName string) {
	lc.WindowName = windowName
}

func (lc *ListContext) GetWindowName() string {
	windowName := lc.WindowName

	if windowName != "" {
		return windowName
	}

	// TODO: actually set this for everything so we don't default to the view name
	return lc.ViewName
}

func (lc *ListContext) SetParentContext(c Context) {
	lc.ParentContext = c
}

func (lc *ListContext) GetParentContext() Context {
	return lc.ParentContext
}

func (lc *ListContext) GetSelectedItemId() string {
	item := lc.SelectedItem()

	if item == nil {
		return ""
	}

	return item.ID()
}

// OnFocus assumes that the content of the context has already been rendered to the view. OnRender is the function which actually renders the content to the view
func (lc *ListContext) OnRender() error {
	view, err := lc.Gui.g.View(lc.ViewName)
	if err != nil {
		return nil
	}

	if lc.GetDisplayStrings != nil {
		lc.Gui.refreshSelectedLine(lc.GetPanelState(), lc.GetItemsLength())
		lc.Gui.renderDisplayStrings(view, lc.GetDisplayStrings())
	}

	return nil
}

func (lc *ListContext) GetKey() string {
	return lc.ContextKey
}

func (lc *ListContext) GetKind() int {
	return lc.Kind
}

func (lc *ListContext) GetViewName() string {
	return lc.ViewName
}

func (lc *ListContext) HandleFocusLost() error {
	if lc.OnFocusLost != nil {
		return lc.OnFocusLost()
	}

	return nil
}

func (lc *ListContext) HandleFocus() error {
	if lc.Gui.popupPanelFocused() {
		return nil
	}

	view, err := lc.Gui.g.View(lc.ViewName)
	if err != nil {
		return nil
	}

	view.FocusPoint(0, lc.GetPanelState().GetSelectedLineIdx())

	if lc.Gui.State.Modes.Diffing.Active() {
		return lc.Gui.renderDiff()
	}

	// every time you select an item we need to store that item's ID on the context (a string). After a state refresh, after we update the selected line, we need to check if the selected item is new, in which case we will reset the origin. In the case of the merge panel we set the origin in a custom way, so it can't be as simple as just resetting the origin. for files we need to know whether we're dealing with a file with merge conflicts, and if so, we need to scroll to the file in a custom way, after rendering to the main view.

	// we can use this id to know what to do once we're actually in the merging context, so that we're not affected by outside state changes.

	if lc.OnFocus != nil {
		return lc.OnFocus()
	}

	return nil
}

func (lc *ListContext) HandleRender() error {
	return lc.OnRender()
}

func (lc *ListContext) handlePrevLine(g *gocui.Gui, v *gocui.View) error {
	return lc.handleLineChange(-1)
}

func (lc *ListContext) handleNextLine(g *gocui.Gui, v *gocui.View) error {
	return lc.handleLineChange(1)
}

func (lc *ListContext) handleLineChange(change int) error {
	if !lc.Gui.isPopupPanel(lc.ViewName) && lc.Gui.popupPanelFocused() {
		return nil
	}

	view, err := lc.Gui.g.View(lc.ViewName)
	if err != nil {
		return err
	}

	lc.Gui.changeSelectedLine(lc.GetPanelState(), lc.GetItemsLength(), change)
	view.FocusPoint(0, lc.GetPanelState().GetSelectedLineIdx())

	if lc.RendersToMainView {
		if err := lc.Gui.resetOrigin(lc.Gui.getMainView()); err != nil {
			return err
		}
		if err := lc.Gui.resetOrigin(lc.Gui.getSecondaryView()); err != nil {
			return err
		}
	}

	return lc.HandleFocus()
}

func (lc *ListContext) handleNextPage(g *gocui.Gui, v *gocui.View) error {
	view, err := lc.Gui.g.View(lc.ViewName)
	if err != nil {
		return nil
	}
	_, height := view.Size()
	delta := height - 1
	if delta == 0 {
		delta = 1
	}
	return lc.handleLineChange(delta)
}

func (lc *ListContext) handleGotoTop(g *gocui.Gui, v *gocui.View) error {
	return lc.handleLineChange(-lc.GetItemsLength())
}

func (lc *ListContext) handleGotoBottom(g *gocui.Gui, v *gocui.View) error {
	return lc.handleLineChange(lc.GetItemsLength())
}

func (lc *ListContext) handlePrevPage(g *gocui.Gui, v *gocui.View) error {
	view, err := lc.Gui.g.View(lc.ViewName)
	if err != nil {
		return nil
	}
	_, height := view.Size()
	delta := height - 1
	if delta == 0 {
		delta = 1
	}
	return lc.handleLineChange(-delta)
}

func (lc *ListContext) handleClick(g *gocui.Gui, v *gocui.View) error {
	if !lc.Gui.isPopupPanel(lc.ViewName) && lc.Gui.popupPanelFocused() {
		return nil
	}

	prevSelectedLineIdx := lc.GetPanelState().GetSelectedLineIdx()
	newSelectedLineIdx := v.SelectedLineIdx()

	// we need to focus the view
	if err := lc.Gui.switchContext(lc); err != nil {
		return err
	}

	if newSelectedLineIdx > lc.GetItemsLength()-1 {
		return nil
	}

	lc.GetPanelState().SetSelectedLineIdx(newSelectedLineIdx)

	prevViewName := lc.Gui.currentViewName()
	if prevSelectedLineIdx == newSelectedLineIdx && prevViewName == lc.ViewName && lc.OnClickSelectedItem != nil {
		return lc.OnClickSelectedItem()
	}
	return lc.HandleFocus()
}

func (lc *ListContext) onSearchSelect(selectedLineIdx int) error {
	lc.GetPanelState().SetSelectedLineIdx(selectedLineIdx)
	return lc.HandleFocus()
}

func (gui *Gui) menuListContext() *ListContext {
	return &ListContext{
		ViewName:       "menu",
		ContextKey:     "menu",
		GetItemsLength: func() int { return gui.getMenuView().LinesHeight() },
		GetPanelState:  func() IListPanelState { return gui.State.Panels.Menu },
		OnFocus:        gui.handleMenuSelect,
		// need to add a layer of indirection here because the callback changes during runtime
		OnClickSelectedItem: func() error { return gui.State.Panels.Menu.OnPress(gui.g, nil) },
		Gui:                 gui,
		RendersToMainView:   false,
		Kind:                PERSISTENT_POPUP,
		Contains:            CONTAINS_NOTHING,

		// no GetDisplayStrings field because we do a custom render on menu creation
	}
}

func (gui *Gui) filesListContext() *ListContext {
	return &ListContext{
		ViewName:            "files",
		ContextKey:          FILES_CONTEXT_KEY,
		GetItemsLength:      func() int { return len(gui.State.Files) },
		GetPanelState:       func() IListPanelState { return gui.State.Panels.Files },
		OnFocus:             gui.focusAndSelectFile,
		OnClickSelectedItem: gui.handleFilePress,
		Gui:                 gui,
		RendersToMainView:   false,
		Kind:                SIDE_CONTEXT,
		GetDisplayStrings: func() [][]string {
			return presentation.GetFileListDisplayStrings(gui.State.Files, gui.State.Modes.Diffing.Ref)
		},
		Contains:     CONTAINS_NOTHING,
		SelectedItem: func() ListItem { return gui.getSelectedFile() },
	}
}

func (gui *Gui) branchesListContext() *ListContext {
	return &ListContext{
		ViewName:          "branches",
		ContextKey:        LOCAL_BRANCHES_CONTEXT_KEY,
		GetItemsLength:    func() int { return len(gui.State.Branches) },
		GetPanelState:     func() IListPanelState { return gui.State.Panels.Branches },
		OnFocus:           gui.handleBranchSelect,
		Gui:               gui,
		RendersToMainView: true,
		Kind:              SIDE_CONTEXT,
		GetDisplayStrings: func() [][]string {
			return presentation.GetBranchListDisplayStrings(gui.State.Branches, gui.State.ScreenMode != SCREEN_NORMAL, gui.State.Modes.Diffing.Ref)
		},
		Contains:     CONTAINS_COMMITS,
		SelectedItem: func() ListItem { return gui.getSelectedBranch() },
	}
}

func (gui *Gui) remotesListContext() *ListContext {
	return &ListContext{
		ViewName:            "branches",
		ContextKey:          REMOTES_CONTEXT_KEY,
		GetItemsLength:      func() int { return len(gui.State.Remotes) },
		GetPanelState:       func() IListPanelState { return gui.State.Panels.Remotes },
		OnFocus:             gui.handleRemoteSelect,
		OnClickSelectedItem: gui.handleRemoteEnter,
		Gui:                 gui,
		RendersToMainView:   true,
		Kind:                SIDE_CONTEXT,
		GetDisplayStrings: func() [][]string {
			return presentation.GetRemoteListDisplayStrings(gui.State.Remotes, gui.State.Modes.Diffing.Ref)
		},
		Contains:     CONTAINS_BRANCHES,
		SelectedItem: func() ListItem { return gui.getSelectedRemote() },
	}
}

func (gui *Gui) remoteBranchesListContext() *ListContext {
	return &ListContext{
		ViewName:          "branches",
		ContextKey:        REMOTE_BRANCHES_CONTEXT_KEY,
		GetItemsLength:    func() int { return len(gui.State.RemoteBranches) },
		GetPanelState:     func() IListPanelState { return gui.State.Panels.RemoteBranches },
		OnFocus:           gui.handleRemoteBranchSelect,
		Gui:               gui,
		RendersToMainView: true,
		Kind:              SIDE_CONTEXT,
		GetDisplayStrings: func() [][]string {
			return presentation.GetRemoteBranchListDisplayStrings(gui.State.RemoteBranches, gui.State.Modes.Diffing.Ref)
		},
		Contains:     CONTAINS_COMMITS,
		SelectedItem: func() ListItem { return gui.getSelectedRemoteBranch() },
	}
}

func (gui *Gui) tagsListContext() *ListContext {
	return &ListContext{
		ViewName:          "branches",
		ContextKey:        TAGS_CONTEXT_KEY,
		GetItemsLength:    func() int { return len(gui.State.Tags) },
		GetPanelState:     func() IListPanelState { return gui.State.Panels.Tags },
		OnFocus:           gui.handleTagSelect,
		Gui:               gui,
		RendersToMainView: true,
		Kind:              SIDE_CONTEXT,
		GetDisplayStrings: func() [][]string {
			return presentation.GetTagListDisplayStrings(gui.State.Tags, gui.State.Modes.Diffing.Ref)
		},
		Contains:     CONTAINS_COMMITS,
		SelectedItem: func() ListItem { return gui.getSelectedTag() },
	}
}

func (gui *Gui) branchCommitsListContext() *ListContext {
	return &ListContext{
		ViewName:            "commits",
		ContextKey:          BRANCH_COMMITS_CONTEXT_KEY,
		GetItemsLength:      func() int { return len(gui.State.Commits) },
		GetPanelState:       func() IListPanelState { return gui.State.Panels.Commits },
		OnFocus:             gui.handleCommitSelect,
		OnClickSelectedItem: gui.handleViewCommitFiles,
		Gui:                 gui,
		RendersToMainView:   true,
		Kind:                SIDE_CONTEXT,
		GetDisplayStrings: func() [][]string {
			return presentation.GetCommitListDisplayStrings(gui.State.Commits, gui.State.ScreenMode != SCREEN_NORMAL, gui.cherryPickedCommitShaMap(), gui.State.Modes.Diffing.Ref)
		},
		Contains:     CONTAINS_FILES,
		SelectedItem: func() ListItem { return gui.getSelectedLocalCommit() },
	}
}

func (gui *Gui) reflogCommitsListContext() *ListContext {
	return &ListContext{
		ViewName:          "commits",
		ContextKey:        REFLOG_COMMITS_CONTEXT_KEY,
		GetItemsLength:    func() int { return len(gui.State.FilteredReflogCommits) },
		GetPanelState:     func() IListPanelState { return gui.State.Panels.ReflogCommits },
		OnFocus:           gui.handleReflogCommitSelect,
		Gui:               gui,
		RendersToMainView: true,
		Kind:              SIDE_CONTEXT,
		GetDisplayStrings: func() [][]string {
			return presentation.GetReflogCommitListDisplayStrings(gui.State.FilteredReflogCommits, gui.State.ScreenMode != SCREEN_NORMAL, gui.cherryPickedCommitShaMap(), gui.State.Modes.Diffing.Ref)
		},
		Contains:     CONTAINS_FILES,
		SelectedItem: func() ListItem { return gui.getSelectedReflogCommit() },
	}
}

func (gui *Gui) subCommitsListContext() *ListContext {
	return &ListContext{
		ViewName:          "branches",
		ContextKey:        SUB_COMMITS_CONTEXT_KEY,
		GetItemsLength:    func() int { return len(gui.State.SubCommits) },
		GetPanelState:     func() IListPanelState { return gui.State.Panels.SubCommits },
		OnFocus:           gui.handleSubCommitSelect,
		Gui:               gui,
		RendersToMainView: true,
		Kind:              SIDE_CONTEXT,
		GetDisplayStrings: func() [][]string {
			gui.Log.Warn("getting display strings for sub commits")
			return presentation.GetCommitListDisplayStrings(gui.State.SubCommits, gui.State.ScreenMode != SCREEN_NORMAL, gui.cherryPickedCommitShaMap(), gui.State.Modes.Diffing.Ref)
		},
		Contains:     CONTAINS_COMMITS,
		SelectedItem: func() ListItem { return gui.getSelectedSubCommit() },
	}
}

func (gui *Gui) stashListContext() *ListContext {
	return &ListContext{
		ViewName:          "stash",
		ContextKey:        STASH_CONTEXT_KEY,
		GetItemsLength:    func() int { return len(gui.State.StashEntries) },
		GetPanelState:     func() IListPanelState { return gui.State.Panels.Stash },
		OnFocus:           gui.handleStashEntrySelect,
		Gui:               gui,
		RendersToMainView: true,
		Kind:              SIDE_CONTEXT,
		GetDisplayStrings: func() [][]string {
			return presentation.GetStashEntryListDisplayStrings(gui.State.StashEntries, gui.State.Modes.Diffing.Ref)
		},
		Contains:     CONTAINS_FILES,
		SelectedItem: func() ListItem { return gui.getSelectedStashEntry() },
	}
}

func (gui *Gui) commitFilesListContext() *ListContext {
	return &ListContext{
		ViewName:          "commitFiles",
		WindowName:        "commits",
		ContextKey:        COMMIT_FILES_CONTEXT_KEY,
		GetItemsLength:    func() int { return len(gui.State.CommitFiles) },
		GetPanelState:     func() IListPanelState { return gui.State.Panels.CommitFiles },
		OnFocus:           gui.handleCommitFileSelect,
		Gui:               gui,
		RendersToMainView: true,
		Kind:              SIDE_CONTEXT,
		GetDisplayStrings: func() [][]string {
			return presentation.GetCommitFileListDisplayStrings(gui.State.CommitFiles, gui.State.Modes.Diffing.Ref)
		},
		Contains:     CONTAINS_NOTHING,
		SelectedItem: func() ListItem { return gui.getSelectedCommitFile() },
	}
}

func (gui *Gui) getListContexts() []*ListContext {
	return []*ListContext{
		gui.menuListContext(),
		gui.filesListContext(),
		gui.branchesListContext(),
		gui.remotesListContext(),
		gui.remoteBranchesListContext(),
		gui.tagsListContext(),
		gui.branchCommitsListContext(),
		gui.reflogCommitsListContext(),
		gui.subCommitsListContext(),
		gui.stashListContext(),
		gui.commitFilesListContext(),
	}
}

func (gui *Gui) getListContextKeyBindings() []*Binding {
	bindings := make([]*Binding, 0)

	for _, listContext := range gui.getListContexts() {
		bindings = append(bindings, []*Binding{
			{ViewName: listContext.ViewName, Contexts: []string{listContext.ContextKey}, Key: gui.getKey("universal.prevItem-alt"), Modifier: gocui.ModNone, Handler: listContext.handlePrevLine},
			{ViewName: listContext.ViewName, Contexts: []string{listContext.ContextKey}, Key: gui.getKey("universal.prevItem"), Modifier: gocui.ModNone, Handler: listContext.handlePrevLine},
			{ViewName: listContext.ViewName, Contexts: []string{listContext.ContextKey}, Key: gocui.MouseWheelUp, Modifier: gocui.ModNone, Handler: listContext.handlePrevLine},
			{ViewName: listContext.ViewName, Contexts: []string{listContext.ContextKey}, Key: gui.getKey("universal.nextItem-alt"), Modifier: gocui.ModNone, Handler: listContext.handleNextLine},
			{ViewName: listContext.ViewName, Contexts: []string{listContext.ContextKey}, Key: gui.getKey("universal.nextItem"), Modifier: gocui.ModNone, Handler: listContext.handleNextLine},
			{ViewName: listContext.ViewName, Contexts: []string{listContext.ContextKey}, Key: gui.getKey("universal.prevPage"), Modifier: gocui.ModNone, Handler: listContext.handlePrevPage, Description: gui.Tr.SLocalize("prevPage")},
			{ViewName: listContext.ViewName, Contexts: []string{listContext.ContextKey}, Key: gui.getKey("universal.nextPage"), Modifier: gocui.ModNone, Handler: listContext.handleNextPage, Description: gui.Tr.SLocalize("nextPage")},
			{ViewName: listContext.ViewName, Contexts: []string{listContext.ContextKey}, Key: gui.getKey("universal.gotoTop"), Modifier: gocui.ModNone, Handler: listContext.handleGotoTop, Description: gui.Tr.SLocalize("gotoTop")},
			{ViewName: listContext.ViewName, Contexts: []string{listContext.ContextKey}, Key: gocui.MouseWheelDown, Modifier: gocui.ModNone, Handler: listContext.handleNextLine},
			{ViewName: listContext.ViewName, Contexts: []string{listContext.ContextKey}, Key: gocui.MouseLeft, Modifier: gocui.ModNone, Handler: listContext.handleClick},
		}...)

		// the commits panel needs to lazyload things so it has a couple of its own handlers
		openSearchHandler := gui.handleOpenSearch
		gotoBottomHandler := listContext.handleGotoBottom
		if listContext.ViewName == "commits" {
			openSearchHandler = gui.handleOpenSearchForCommitsPanel
			gotoBottomHandler = gui.handleGotoBottomForCommitsPanel
		}

		bindings = append(bindings, []*Binding{
			{
				ViewName:    listContext.ViewName,
				Contexts:    []string{listContext.ContextKey},
				Key:         gui.getKey("universal.startSearch"),
				Handler:     openSearchHandler,
				Description: gui.Tr.SLocalize("startSearch"),
			},
			{
				ViewName:    listContext.ViewName,
				Contexts:    []string{listContext.ContextKey},
				Key:         gui.getKey("universal.gotoBottom"),
				Handler:     gotoBottomHandler,
				Description: gui.Tr.SLocalize("gotoBottom"),
			},
		}...)
	}

	return bindings
}