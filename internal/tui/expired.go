package tui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/joaoalvarenga/dinha/internal/model"
	"github.com/joaoalvarenga/dinha/internal/service"
)

type expiredSortColumn int

const (
	expSortByName expiredSortColumn = iota
	expSortBySize
	expSortByLastActivity
	expSortByExpiredAt
	expSortByExpiredFor
)

type expiredEntry struct {
	file         model.File
	size         int64
	lastActivity time.Time
}

// expiredTreeNode represents a directory in the virtual tree of expired files.
type expiredTreeNode struct {
	children   map[string]*expiredTreeNode
	files      []expiredEntry // expired files directly in this dir
	totalCount int            // total recursive count of expired files
	totalSize  int64          // total recursive size
}

// expiredDisplayItem is a row in the table (directory or file).
type expiredDisplayItem struct {
	name         string
	fullPath     string
	isDir        bool
	size         int64
	lastActivity time.Time
	expiredAt    time.Time
	expiredCount int // only for dirs
}

func newExpiredTreeNode() *expiredTreeNode {
	return &expiredTreeNode{children: make(map[string]*expiredTreeNode)}
}

// buildExpiredTree constructs a virtual directory tree from a flat list of expired entries.
func buildExpiredTree(entries []expiredEntry) *expiredTreeNode {
	root := newExpiredTreeNode()
	for _, e := range entries {
		parts := strings.Split(e.file.AbsoluteFilePath, string(os.PathSeparator))
		// Remove empty parts (leading slash produces empty first element)
		var cleaned []string
		for _, p := range parts {
			if p != "" {
				cleaned = append(cleaned, p)
			}
		}
		if len(cleaned) == 0 {
			continue
		}
		// Walk to parent directory, creating nodes as needed
		node := root
		dirParts := cleaned[:len(cleaned)-1]
		for _, p := range dirParts {
			if _, ok := node.children[p]; !ok {
				node.children[p] = newExpiredTreeNode()
			}
			node = node.children[p]
		}
		node.files = append(node.files, e)
	}
	// Compute totals bottom-up
	computeTotals(root)
	return root
}

func computeTotals(node *expiredTreeNode) (int, int64) {
	count := len(node.files)
	var size int64
	for _, f := range node.files {
		size += f.size
	}
	for _, child := range node.children {
		cc, cs := computeTotals(child)
		count += cc
		size += cs
	}
	node.totalCount = count
	node.totalSize = size
	return count, size
}

// findVirtualRoot collapses single-child directories with no files.
func findVirtualRoot(root *expiredTreeNode) (string, *expiredTreeNode) {
	var pathParts []string
	node := root
	for len(node.files) == 0 && len(node.children) == 1 {
		for name, child := range node.children {
			pathParts = append(pathParts, name)
			node = child
		}
	}
	path := string(os.PathSeparator) + strings.Join(pathParts, string(os.PathSeparator))
	return path, node
}

// resolveNode walks the tree from virtualRoot to reach the node at the given absolute path.
func resolveNode(root *expiredTreeNode, virtualRootPath, targetPath string) *expiredTreeNode {
	if targetPath == virtualRootPath {
		return root
	}
	rel, err := filepath.Rel(virtualRootPath, targetPath)
	if err != nil {
		return nil
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	node := root
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		child, ok := node.children[p]
		if !ok {
			return nil
		}
		node = child
	}
	return node
}

func (a *App) showExpiredFiles() {
	currentSort := expSortByExpiredFor
	sortAsc := false

	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetBorders(false).
		SetSeparator(' ')

	pathLabel := tview.NewTextView().
		SetDynamicColors(true)
	pathLabel.SetBackgroundColor(tcell.ColorDefault)

	pathDetail := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	pathDetail.SetBackgroundColor(tcell.ColorDefault)

	summaryLabel := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	summaryLabel.SetBackgroundColor(tcell.ColorDefault)

	helpText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(
			"[blue::b]Enter[-::-] Open  " +
				"[blue::b]d[-::-] Delete  " +
				"[blue::b]Esc[-::-] Back  " +
				"[blue::b]1-5[-::-] Sort",
		)
	helpText.SetBackgroundColor(tcell.ColorDefault)

	headerNames := []string{"NAME", "SIZE", "LAST ACTIVITY", "EXPIRED AT", "EXPIRED FOR"}

	// State
	var virtualRootPath string
	var virtualRoot *expiredTreeNode
	var currentPath string
	var displayItems []expiredDisplayItem
	var allEntries []expiredEntry

	setHeaders := func() {
		for i, name := range headerNames {
			label := name
			if expiredSortColumn(i) == currentSort {
				if sortAsc {
					label += " ▲"
				} else {
					label += " ▼"
				}
			}
			expansion := 0
			maxWidth := 0
			switch i {
			case 0:
				expansion = 1
			case 1:
				maxWidth = 12
			case 2:
				maxWidth = 20
			case 3:
				maxWidth = 20
			case 4:
				maxWidth = 14
			}
			cell := tview.NewTableCell(label).
				SetSelectable(false).
				SetTextColor(tcell.ColorRed).
				SetAttributes(tcell.AttrBold).
				SetExpansion(expansion)
			if maxWidth > 0 {
				cell.SetMaxWidth(maxWidth)
			}
			table.SetCell(0, i, cell)
		}
	}

	sortDisplayItems := func() {
		now := time.Now()
		sort.SliceStable(displayItems, func(i, j int) bool {
			// Directories always first
			if displayItems[i].isDir != displayItems[j].isDir {
				return displayItems[i].isDir
			}

			var less bool
			switch currentSort {
			case expSortByName:
				less = strings.ToLower(displayItems[i].name) < strings.ToLower(displayItems[j].name)
			case expSortBySize:
				less = displayItems[i].size < displayItems[j].size
			case expSortByLastActivity:
				if displayItems[i].isDir || displayItems[j].isDir {
					less = strings.ToLower(displayItems[i].name) < strings.ToLower(displayItems[j].name)
				} else {
					less = displayItems[i].lastActivity.Before(displayItems[j].lastActivity)
				}
			case expSortByExpiredAt:
				if displayItems[i].isDir || displayItems[j].isDir {
					less = strings.ToLower(displayItems[i].name) < strings.ToLower(displayItems[j].name)
				} else {
					less = displayItems[i].expiredAt.Before(displayItems[j].expiredAt)
				}
			case expSortByExpiredFor:
				if displayItems[i].isDir || displayItems[j].isDir {
					less = strings.ToLower(displayItems[i].name) < strings.ToLower(displayItems[j].name)
				} else {
					less = now.Sub(displayItems[i].expiredAt) > now.Sub(displayItems[j].expiredAt)
				}
			default:
				less = strings.ToLower(displayItems[i].name) < strings.ToLower(displayItems[j].name)
			}

			if !sortAsc {
				return !less
			}
			return less
		})
	}

	renderTable := func() {
		table.Clear()
		setHeaders()

		if len(displayItems) == 0 {
			table.SetCell(1, 0, tview.NewTableCell("  No expired files").
				SetTextColor(tcell.ColorGreen).
				SetSelectable(false).
				SetExpansion(1))
			table.Select(1, 0)
			pathDetail.SetText("")
			return
		}

		now := time.Now()

		for i, item := range displayItems {
			row := i + 1

			if item.isDir {
				label := fmt.Sprintf("  📁 %s (%d files)", item.name, item.expiredCount)
				table.SetCell(row, 0, tview.NewTableCell(label).
					SetTextColor(tcell.ColorDodgerBlue).
					SetExpansion(1))

				table.SetCell(row, 1, tview.NewTableCell(humanSize(item.size)).
					SetTextColor(sizeColor(item.size)).
					SetMaxWidth(12).
					SetAlign(tview.AlignRight))

				table.SetCell(row, 2, tview.NewTableCell("-").
					SetTextColor(tcell.ColorGray).
					SetMaxWidth(20))
				table.SetCell(row, 3, tview.NewTableCell("-").
					SetTextColor(tcell.ColorGray).
					SetMaxWidth(20))
				table.SetCell(row, 4, tview.NewTableCell("-").
					SetTextColor(tcell.ColorGray).
					SetMaxWidth(14))
			} else {
				table.SetCell(row, 0, tview.NewTableCell("  📄 "+item.name).
					SetTextColor(tcell.ColorRed).
					SetExpansion(1))

				table.SetCell(row, 1, tview.NewTableCell(humanSize(item.size)).
					SetTextColor(tcell.ColorRed).
					SetMaxWidth(12).
					SetAlign(tview.AlignRight))

				table.SetCell(row, 2, tview.NewTableCell(item.lastActivity.Format("2006-01-02 15:04")).
					SetTextColor(tcell.ColorRed).
					SetMaxWidth(20))

				table.SetCell(row, 3, tview.NewTableCell(item.expiredAt.Format("2006-01-02 15:04")).
					SetTextColor(tcell.ColorRed).
					SetMaxWidth(20))

				expiredFor := now.Sub(item.expiredAt)
				table.SetCell(row, 4, tview.NewTableCell(humanAge(expiredFor)).
					SetTextColor(tcell.ColorRed).
					SetMaxWidth(14))
			}
		}

		table.Select(1, 0)
		table.ScrollToBeginning()
	}

	populateTable := func(dir string) {
		currentPath = dir

		node := resolveNode(virtualRoot, virtualRootPath, currentPath)
		if node == nil {
			currentPath = virtualRootPath
			node = virtualRoot
		}

		// Breadcrumb
		rel, _ := filepath.Rel(virtualRootPath, currentPath)
		if rel == "." {
			rel = ""
		}
		breadcrumb := "[yellow::b]" + virtualRootPath + "[-::-]"
		if rel != "" {
			breadcrumb += "[white] / " + strings.ReplaceAll(rel, string(os.PathSeparator), " / ") + "[-]"
		}
		pathLabel.SetText(breadcrumb)

		// Build display items
		displayItems = nil

		// Add subdirectories
		for name, child := range node.children {
			if child.totalCount == 0 {
				continue
			}
			displayItems = append(displayItems, expiredDisplayItem{
				name:         name,
				fullPath:     filepath.Join(currentPath, name),
				isDir:        true,
				size:         child.totalSize,
				expiredCount: child.totalCount,
			})
		}

		// Add files
		for _, e := range node.files {
			displayItems = append(displayItems, expiredDisplayItem{
				name:         filepath.Base(e.file.AbsoluteFilePath),
				fullPath:     e.file.AbsoluteFilePath,
				isDir:        false,
				size:         e.size,
				lastActivity: e.lastActivity,
				expiredAt:    e.file.Expiration.Time,
			})
		}

		sortDisplayItems()
		renderTable()

		// Summary
		fileCount := len(node.files)
		dirCount := 0
		for _, child := range node.children {
			if child.totalCount > 0 {
				dirCount++
			}
		}
		summaryLabel.SetText(fmt.Sprintf(
			"[white]%d files, %d folders[-] [red]| %d expired total[-]",
			fileCount, dirCount, node.totalCount,
		))
	}

	// Load data and build tree
	loadData := func() {
		allEntries = nil
		expired, err := service.ListExpiredFiles(a.db, false)
		if err != nil {
			log.Printf("error loading expired files: %v", err)
			return
		}
		for _, f := range expired {
			var size int64
			if info, err := os.Stat(f.AbsoluteFilePath); err == nil {
				size = info.Size()
			}
			lastActivity := f.ModifiedAt
			if f.AccessedAt.After(lastActivity) {
				lastActivity = f.AccessedAt
			}
			allEntries = append(allEntries, expiredEntry{
				file:         f,
				size:         size,
				lastActivity: lastActivity,
			})
		}
	}

	rebuildTree := func() {
		tree := buildExpiredTree(allEntries)
		virtualRootPath, virtualRoot = findVirtualRoot(tree)
	}

	loadData()
	rebuildTree()
	populateTable(virtualRootPath)

	// Update path detail on selection change
	table.SetSelectionChangedFunc(func(row, column int) {
		idx := row - 1
		if idx >= 0 && idx < len(displayItems) {
			pathDetail.SetText("[yellow]" + displayItems[idx].fullPath + "[-]")
		} else {
			pathDetail.SetText("")
		}
	})

	// Trigger initial path detail
	if len(displayItems) > 0 {
		pathDetail.SetText("[yellow]" + displayItems[0].fullPath + "[-]")
	}

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			row, _ := table.GetSelection()
			idx := row - 1
			if idx >= 0 && idx < len(displayItems) && displayItems[idx].isDir {
				populateTable(displayItems[idx].fullPath)
			}
			return nil

		case tcell.KeyEscape:
			if currentPath == virtualRootPath {
				a.pages.RemovePage("expired")
				return nil
			}
			parent := filepath.Dir(currentPath)
			if len(parent) < len(virtualRootPath) {
				parent = virtualRootPath
			}
			populateTable(parent)
			return nil
		}

		// Sort by column: 1-5
		if event.Rune() >= '1' && event.Rune() <= '5' {
			col := expiredSortColumn(event.Rune() - '1')
			if col == currentSort {
				sortAsc = !sortAsc
			} else {
				currentSort = col
				sortAsc = true
			}
			sortDisplayItems()
			renderTable()
			return nil
		}

		if event.Rune() == 'd' {
			row, _ := table.GetSelection()
			idx := row - 1
			if idx < 0 || idx >= len(displayItems) {
				return nil
			}
			item := displayItems[idx]
			label := "file"
			if item.isDir {
				label = fmt.Sprintf("folder (%d expired files)", item.expiredCount)
			}
			modal := tview.NewModal().
				SetText(fmt.Sprintf("Delete %s?\n%s", label, item.fullPath)).
				AddButtons([]string{"OK", "Cancel"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					if buttonLabel == "OK" {
						var err error
						if item.isDir {
							err = service.DeletePath(a.db, item.fullPath)
						} else {
							err = service.DeleteFile(a.db, item.fullPath)
						}
						if err != nil {
							log.Printf("error deleting %s: %v", item.fullPath, err)
						}
						// Reload data and rebuild tree in-place (preserves sort & path)
						savedPath := currentPath
						savedRow, _ := table.GetSelection()
						loadData()
						rebuildTree()
						// If current directory no longer exists in tree, fall back to virtual root
						if resolveNode(virtualRoot, virtualRootPath, savedPath) == nil {
							savedPath = virtualRootPath
						}
						populateTable(savedPath)
						// Restore selection row, clamped to valid range
						maxRow := table.GetRowCount() - 1
						if savedRow > maxRow {
							savedRow = maxRow
						}
						if savedRow < 1 {
							savedRow = 1
						}
						table.Select(savedRow, 0)
					}
					a.pages.RemovePage("delete-confirm")
				})
			modal.SetBorderColor(tcell.ColorRed)
			modal.SetBackgroundColor(tcell.ColorBlack)
			modal.SetTextColor(tcell.ColorWhite)
			modal.SetButtonBackgroundColor(tcell.ColorDarkSlateGray)
			modal.SetButtonActivatedStyle(tcell.StyleDefault.Background(tcell.ColorDodgerBlue).Foreground(tcell.ColorWhite))
			a.pages.AddPage("delete-confirm", modal, true, true)
			return nil
		}

		return event
	})

	container := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(pathLabel, 1, 0, false).
		AddItem(table, 0, 1, true).
		AddItem(pathDetail, 1, 0, false).
		AddItem(summaryLabel, 1, 0, false).
		AddItem(helpText, 1, 0, false)

	container.SetBorder(true).
		SetTitle(" Expired Files ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorRed).
		SetTitleColor(tcell.ColorWhite)

	a.pages.AddPage("expired", container, true, true)
}
