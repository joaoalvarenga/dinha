package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) showFileBrowser(initialPath string, onSelect func(path string)) {
	if initialPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			initialPath = "/"
		} else {
			initialPath = home
		}
	}

	currentPath := initialPath

	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetSelectedBackgroundColor(tcell.ColorDodgerBlue).
		SetSelectedTextColor(tcell.ColorWhite).
		SetMainTextColor(tcell.ColorWhite)

	pathLabel := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[yellow::b]" + currentPath + "[-::-]")
	pathLabel.SetBackgroundColor(tcell.ColorDefault)

	helpText := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[blue::b]Enter[-::-] Open  [blue::b]Backspace[-::-] Parent  [blue::b]s[-::-] Select  [blue::b]Esc[-::-] Cancel")
	helpText.SetBackgroundColor(tcell.ColorDefault)

	var populateList func(dir string)
	populateList = func(dir string) {
		list.Clear()
		currentPath = dir
		pathLabel.SetText("[yellow::b]" + currentPath + "[-::-]")

		entries, err := os.ReadDir(dir)
		if err != nil {
			list.AddItem("[red]Error reading directory[-]", "", 0, nil)
			return
		}

		// Separate and sort: dirs first
		var dirs []os.DirEntry
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				dirs = append(dirs, e)
			}
		}
		sort.Slice(dirs, func(i, j int) bool {
			return strings.ToLower(dirs[i].Name()) < strings.ToLower(dirs[j].Name())
		})

		for _, d := range dirs {
			name := d.Name()
			list.AddItem("📁 "+name, "", 0, nil)
		}

		if list.GetItemCount() == 0 {
			list.AddItem("[gray](empty)[-]", "", 0, nil)
		}
	}

	getDirName := func(index int) string {
		if index < 0 || index >= list.GetItemCount() {
			return ""
		}
		text, _ := list.GetItemText(index)
		// Remove the folder icon prefix
		text = strings.TrimPrefix(text, "📁 ")
		return text
	}

	navigateInto := func() {
		idx := list.GetCurrentItem()
		dirName := getDirName(idx)
		if dirName == "" || dirName == "[gray](empty)[-]" {
			return
		}
		target := filepath.Join(currentPath, dirName)
		info, err := os.Stat(target)
		if err != nil || !info.IsDir() {
			return
		}
		populateList(target)
	}

	navigateUp := func() {
		parent := filepath.Dir(currentPath)
		if parent == currentPath {
			return // already at root
		}
		populateList(parent)
	}

	selectCurrent := func() {
		onSelect(currentPath)
		a.pages.RemovePage("filebrowser")
	}

	cancel := func() {
		a.pages.RemovePage("filebrowser")
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			navigateInto()
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			navigateUp()
			return nil
		case tcell.KeyEscape:
			cancel()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 's':
				selectCurrent()
				return nil
			}
		}
		return event
	})

	populateList(currentPath)

	// Layout
	container := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(pathLabel, 1, 0, false).
		AddItem(list, 0, 1, true).
		AddItem(helpText, 1, 0, false)

	container.SetBorder(true).
		SetTitle(" Select Folder ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorDodgerBlue).
		SetTitleColor(tcell.ColorWhite)

	// Center the browser as an overlay
	centered := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(container, 25, 0, true).
				AddItem(nil, 0, 1, false),
			80, 0, true,
		).
		AddItem(nil, 0, 1, false)

	a.pages.AddPage("filebrowser", centered, true, true)
}
