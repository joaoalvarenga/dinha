package tui

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/joaoalvarenga/dinha/internal/service"
)

type sortColumn int

const (
	sortByName sortColumn = iota
	sortBySize
	sortByModified
	sortByModAge
	sortByAccessAge
)

func (a *App) showWatchExplorer(watchRoot string) {
	currentPath := watchRoot
	currentSort := sortByName
	sortAsc := true

	table := tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetBorders(false).
		SetSeparator(' ')

	pathLabel := tview.NewTextView().
		SetDynamicColors(true)
	pathLabel.SetBackgroundColor(tcell.ColorDefault)

	summaryLabel := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	summaryLabel.SetBackgroundColor(tcell.ColorDefault)

	helpText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(
			"[blue::b]Enter[-::-] Open  " +
				"[blue::b]Space[-::-] Select  " +
				"[blue::b]d[-::-] Delete  " +
				"[blue::b]Esc[-::-] Back  " +
				"[blue::b]1-5[-::-] Sort",
		)
	helpText.SetBackgroundColor(tcell.ColorDefault)

	type fileEntry struct {
		name       string
		isDir      bool
		size       int64
		modTime    time.Time
		accessTime time.Time
		expired    bool
	}

	headerNames := []string{"NAME", "SIZE", "MODIFIED", "MOD AGE", "ACCESS AGE"}

	setHeaders := func() {
		for i, name := range headerNames {
			label := name
			if sortColumn(i) == currentSort {
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
				maxWidth = 14
			case 4:
				maxWidth = 14
			}
			cell := tview.NewTableCell(label).
				SetSelectable(false).
				SetTextColor(tcell.ColorDodgerBlue).
				SetAttributes(tcell.AttrBold).
				SetExpansion(expansion)
			if maxWidth > 0 {
				cell.SetMaxWidth(maxWidth)
			}
			table.SetCell(0, i, cell)
		}
	}

	sortItems := func(items []fileEntry) {
		now := time.Now()
		sort.SliceStable(items, func(i, j int) bool {
			// Directories always first
			if items[i].isDir != items[j].isDir {
				return items[i].isDir
			}

			var less bool
			switch currentSort {
			case sortByName:
				less = strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
			case sortBySize:
				less = items[i].size < items[j].size
			case sortByModified:
				less = items[i].modTime.Before(items[j].modTime)
			case sortByModAge:
				// Older = larger age = larger duration
				less = now.Sub(items[i].modTime) > now.Sub(items[j].modTime)
			case sortByAccessAge:
				less = now.Sub(items[i].accessTime) > now.Sub(items[j].accessTime)
			default:
				less = strings.ToLower(items[i].name) < strings.ToLower(items[j].name)
			}

			if !sortAsc {
				return !less
			}
			return less
		})
	}

	var currentItems []fileEntry
	selected := make(map[string]bool) // selected item names

	renderTable := func() {
		table.Clear()
		setHeaders()

		now := time.Now()
		for i, item := range currentItems {
			row := i + 1

			// Name with icon and selection marker
			var icon string
			if selected[item.name] {
				if item.isDir {
					icon = " ✓📁 "
				} else {
					icon = " ✓📄 "
				}
			} else {
				if item.isDir {
					icon = "  📁 "
				} else {
					icon = "  📄 "
				}
			}
			nameColor := tcell.ColorWhite
			if item.isDir {
				nameColor = tcell.ColorDodgerBlue
			}
			if item.expired {
				nameColor = tcell.ColorRed
			}
			if selected[item.name] {
				nameColor = tcell.ColorYellow
			}
			table.SetCell(row, 0, tview.NewTableCell(icon+item.name).
				SetTextColor(nameColor).
				SetExpansion(1))

			// Size
			sizeStr := humanSize(item.size)
			sizeClr := sizeColor(item.size)
			if item.isDir {
				sizeStr = "-"
				sizeClr = tcell.ColorGray
			}
			if item.expired {
				sizeClr = tcell.ColorRed
			}
			if selected[item.name] {
				sizeClr = tcell.ColorYellow
			}
			table.SetCell(row, 1, tview.NewTableCell(sizeStr).
				SetTextColor(sizeClr).
				SetMaxWidth(12).
				SetAlign(tview.AlignRight))

			// Modified date
			modClr := tcell.ColorLightGray
			if item.expired {
				modClr = tcell.ColorRed
			}
			if selected[item.name] {
				modClr = tcell.ColorYellow
			}
			table.SetCell(row, 2, tview.NewTableCell(item.modTime.Format("2006-01-02 15:04")).
				SetTextColor(modClr).
				SetMaxWidth(20))

			// Mod age
			modAge := now.Sub(item.modTime)
			modAgeClr := ageColor(modAge)
			if item.expired {
				modAgeClr = tcell.ColorRed
			}
			if selected[item.name] {
				modAgeClr = tcell.ColorYellow
			}
			table.SetCell(row, 3, tview.NewTableCell(humanAge(modAge)).
				SetTextColor(modAgeClr).
				SetMaxWidth(14))

			// Access age
			accessAge := now.Sub(item.accessTime)
			accessAgeClr := ageColor(accessAge)
			if item.expired {
				accessAgeClr = tcell.ColorRed
			}
			if selected[item.name] {
				accessAgeClr = tcell.ColorYellow
			}
			table.SetCell(row, 4, tview.NewTableCell(humanAge(accessAge)).
				SetTextColor(accessAgeClr).
				SetMaxWidth(14))
		}

		if len(currentItems) == 0 {
			table.SetCell(1, 0, tview.NewTableCell("  (empty)").
				SetTextColor(tcell.ColorGray).
				SetSelectable(false).
				SetExpansion(1))
		}

		table.Select(1, 0)
		table.ScrollToBeginning()
	}

	populateTable := func(dir string) {
		currentPath = dir
		selected = make(map[string]bool)

		// Update path label with breadcrumb
		rel, _ := filepath.Rel(watchRoot, currentPath)
		if rel == "." {
			rel = ""
		}
		breadcrumb := "[yellow::b]" + watchRoot + "[-::-]"
		if rel != "" {
			breadcrumb += "[white] / " + rel + "[-]"
		}
		pathLabel.SetText(breadcrumb)

		entries, err := os.ReadDir(dir)
		if err != nil {
			table.Clear()
			setHeaders()
			table.SetCell(1, 0, tview.NewTableCell("[red]Error reading directory[-]").
				SetExpansion(1))
			summaryLabel.SetText("")
			currentItems = nil
			return
		}

		currentItems = nil
		var totalSize int64
		var fileCount, dirCount int

		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			fe := fileEntry{
				name:    e.Name(),
				isDir:   e.IsDir(),
				modTime: info.ModTime(),
			}
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				fe.accessTime = time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)
			}
			if e.IsDir() {
				fe.size = -1
				dirCount++
			} else {
				fe.size = info.Size()
				fileCount++
				totalSize += fe.size

				fullPath := filepath.Join(dir, e.Name())
				if f, err := service.FindFile(a.db, fullPath); err == nil && f != nil {
					if f.Expiration.Valid && f.Expiration.Time.Before(time.Now()) {
						fe.expired = true
					}
				}
			}
			currentItems = append(currentItems, fe)
		}

		sortItems(currentItems)
		renderTable()

		summaryLabel.SetText(fmt.Sprintf(
			"[white]%d files, %d folders — Total: %s[-]",
			fileCount, dirCount, humanSize(totalSize),
		))
	}

	getEntryName := func(row int) (string, bool) {
		idx := row - 1
		if idx < 0 || idx >= len(currentItems) {
			return "", false
		}
		return currentItems[idx].name, currentItems[idx].isDir
	}

	updateSelectionSummary := func() {
		if len(selected) == 0 {
			return
		}
		var totalSize int64
		for _, item := range currentItems {
			if selected[item.name] {
				if !item.isDir {
					totalSize += item.size
				}
			}
		}
		summaryLabel.SetText(fmt.Sprintf(
			"[yellow::b]%d selected[-::-] [white]— %s[-]",
			len(selected), humanSize(totalSize),
		))
	}

	rerenderKeepSelection := func() {
		savedRow, _ := table.GetSelection()
		table.Clear()
		setHeaders()

		now := time.Now()
		for i, item := range currentItems {
			row := i + 1

			var icon string
			if selected[item.name] {
				if item.isDir {
					icon = " ✓📁 "
				} else {
					icon = " ✓📄 "
				}
			} else {
				if item.isDir {
					icon = "  📁 "
				} else {
					icon = "  📄 "
				}
			}
			nameColor := tcell.ColorWhite
			if item.isDir {
				nameColor = tcell.ColorDodgerBlue
			}
			if item.expired {
				nameColor = tcell.ColorRed
			}
			if selected[item.name] {
				nameColor = tcell.ColorYellow
			}
			table.SetCell(row, 0, tview.NewTableCell(icon+item.name).
				SetTextColor(nameColor).
				SetExpansion(1))

			sizeStr := humanSize(item.size)
			sizeClr := sizeColor(item.size)
			if item.isDir {
				sizeStr = "-"
				sizeClr = tcell.ColorGray
			}
			if item.expired {
				sizeClr = tcell.ColorRed
			}
			if selected[item.name] {
				sizeClr = tcell.ColorYellow
			}
			table.SetCell(row, 1, tview.NewTableCell(sizeStr).
				SetTextColor(sizeClr).
				SetMaxWidth(12).
				SetAlign(tview.AlignRight))

			modClr := tcell.ColorLightGray
			if item.expired {
				modClr = tcell.ColorRed
			}
			if selected[item.name] {
				modClr = tcell.ColorYellow
			}
			table.SetCell(row, 2, tview.NewTableCell(item.modTime.Format("2006-01-02 15:04")).
				SetTextColor(modClr).
				SetMaxWidth(20))

			modAge := now.Sub(item.modTime)
			modAgeClr := ageColor(modAge)
			if item.expired {
				modAgeClr = tcell.ColorRed
			}
			if selected[item.name] {
				modAgeClr = tcell.ColorYellow
			}
			table.SetCell(row, 3, tview.NewTableCell(humanAge(modAge)).
				SetTextColor(modAgeClr).
				SetMaxWidth(14))

			accessAge := now.Sub(item.accessTime)
			accessAgeClr := ageColor(accessAge)
			if item.expired {
				accessAgeClr = tcell.ColorRed
			}
			if selected[item.name] {
				accessAgeClr = tcell.ColorYellow
			}
			table.SetCell(row, 4, tview.NewTableCell(humanAge(accessAge)).
				SetTextColor(accessAgeClr).
				SetMaxWidth(14))
		}

		if len(currentItems) == 0 {
			table.SetCell(1, 0, tview.NewTableCell("  (empty)").
				SetTextColor(tcell.ColorGray).
				SetSelectable(false).
				SetExpansion(1))
		}

		maxRow := table.GetRowCount() - 1
		if savedRow > maxRow {
			savedRow = maxRow
		}
		if savedRow < 1 {
			savedRow = 1
		}
		table.Select(savedRow, 0)
	}

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			row, _ := table.GetSelection()
			name, isDir := getEntryName(row)
			if name != "" && isDir {
				populateTable(filepath.Join(currentPath, name))
			}
			return nil

		case tcell.KeyEscape:
			if len(selected) > 0 {
				selected = make(map[string]bool)
				rerenderKeepSelection()
				updateSelectionSummary()
				return nil
			}
			if currentPath == watchRoot {
				a.pages.RemovePage("explorer")
				return nil
			}
			parent := filepath.Dir(currentPath)
			if len(parent) < len(watchRoot) {
				parent = watchRoot
			}
			populateTable(parent)
			return nil
		}

		if event.Rune() == ' ' {
			row, _ := table.GetSelection()
			name, _ := getEntryName(row)
			if name == "" {
				return nil
			}
			if selected[name] {
				delete(selected, name)
			} else {
				selected[name] = true
			}
			rerenderKeepSelection()
			updateSelectionSummary()
			// Move cursor down
			nextRow := row + 1
			if nextRow < table.GetRowCount() {
				table.Select(nextRow, 0)
			}
			return nil
		}

		if event.Rune() == 'd' {
			// Collect paths to delete
			var paths []string
			if len(selected) > 0 {
				for _, item := range currentItems {
					if selected[item.name] {
						paths = append(paths, filepath.Join(currentPath, item.name))
					}
				}
			} else {
				row, _ := table.GetSelection()
				name, _ := getEntryName(row)
				if name == "" {
					return nil
				}
				paths = append(paths, filepath.Join(currentPath, name))
			}
			if len(paths) == 0 {
				return nil
			}

			var msg string
			if len(paths) == 1 {
				info, err := os.Stat(paths[0])
				label := "file"
				if err == nil && info.IsDir() {
					label = "folder"
				}
				msg = fmt.Sprintf("Delete %s?\n%s", label, paths[0])
			} else {
				msg = fmt.Sprintf("Delete %d items?\n", len(paths))
				for i, p := range paths {
					if i >= 5 {
						msg += fmt.Sprintf("... and %d more", len(paths)-5)
						break
					}
					msg += filepath.Base(p) + "\n"
				}
			}

			modal := tview.NewModal().
				SetText(msg).
				AddButtons([]string{"OK", "Cancel"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					if buttonLabel == "OK" {
						for _, p := range paths {
							if err := service.DeletePath(a.db, p); err != nil {
								log.Printf("error deleting %s: %v", p, err)
							}
						}
						populateTable(currentPath)
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

		// Sort by column: 1-5
		if event.Rune() >= '1' && event.Rune() <= '5' {
			col := sortColumn(event.Rune() - '1')
			if col == currentSort {
				sortAsc = !sortAsc
			} else {
				currentSort = col
				sortAsc = true
			}
			sortItems(currentItems)
			rerenderKeepSelection()
			return nil
		}

		return event
	})

	populateTable(currentPath)

	// Layout
	container := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(pathLabel, 1, 0, false).
		AddItem(table, 0, 1, true).
		AddItem(summaryLabel, 1, 0, false).
		AddItem(helpText, 1, 0, false)

	container.SetBorder(true).
		SetTitle(" Explorer ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorDodgerBlue).
		SetTitleColor(tcell.ColorWhite)

	a.pages.AddPage("explorer", container, true, true)
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffixes := []string{"KB", "MB", "GB", "TB"}
	val := float64(b) / float64(div)
	if val >= 100 {
		return fmt.Sprintf("%.0f %s", val, suffixes[exp])
	}
	if val >= 10 {
		return fmt.Sprintf("%.1f %s", val, suffixes[exp])
	}
	return fmt.Sprintf("%.2f %s", val, suffixes[exp])
}

func sizeColor(b int64) tcell.Color {
	switch {
	case b >= 1<<30: // >= 1 GB
		return tcell.ColorRed
	case b >= 100<<20: // >= 100 MB
		return tcell.ColorOrange
	case b >= 10<<20: // >= 10 MB
		return tcell.ColorYellow
	default:
		return tcell.ColorLightGray
	}
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	case d < 365*24*time.Hour:
		months := int(d.Hours() / 24 / 30)
		return fmt.Sprintf("%dmo ago", months)
	default:
		years := int(d.Hours() / 24 / 365)
		return fmt.Sprintf("%dy ago", years)
	}
}

func ageColor(d time.Duration) tcell.Color {
	switch {
	case d >= 365*24*time.Hour:
		return tcell.ColorRed
	case d >= 90*24*time.Hour:
		return tcell.ColorOrange
	case d >= 30*24*time.Hour:
		return tcell.ColorYellow
	case d >= 7*24*time.Hour:
		return tcell.ColorLightGray
	default:
		return tcell.ColorGreen
	}
}
