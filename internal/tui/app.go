package tui

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/joaoalvarenga/dinha/internal/service"
	"github.com/joaoalvarenga/dinha/internal/status"
)

var durationRegex = regexp.MustCompile(`^(\d+)([smhdM])$`)

type App struct {
	db           *sql.DB
	app          *tview.Application
	pages        *tview.Pages
	table        *tview.Table
	daemonStatus *tview.TextView
}

func New(db *sql.DB) *App {
	a := &App{db: db}
	a.build()
	return a
}

func (a *App) Run() error {
	go a.refreshLoop()
	return a.app.Run()
}

func (a *App) build() {
	a.app = tview.NewApplication()

	// Banner
	banner := tview.NewTextView().
		SetText(Banner).
		SetTextColor(tcell.ColorDodgerBlue).
		SetDynamicColors(false)

	// Commands help
	commands := tview.NewTextView().
		SetDynamicColors(true).
		SetText(
			"[blue::b]<q>[-::-] Quit\n" +
				"[blue::b]<Enter>[-::-] Explore folder\n" +
				"[blue::b]<a>[-::-] Watch new folder\n" +
				"[blue::b]<d>[-::-] Unwatch folder\n" +
				"[blue::b]<e>[-::-] Edit expiration date\n" +
				"[blue::b]<x>[-::-] View expired files\n" +
				"[blue::b]<s>[-::-] Daemon settings",
		)

	// Daemon status
	a.daemonStatus = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	a.daemonStatus.SetBackgroundColor(tcell.ColorDefault)
	a.refreshDaemonStatus()

	header := tview.NewFlex().
		AddItem(banner, 38, 0, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(commands, 0, 1, false).
				AddItem(a.daemonStatus, 1, 0, false),
			0, 1, false,
		)

	// Table
	a.table = tview.NewTable().
		SetSelectable(true, false).
		SetFixed(1, 0).
		SetBorders(false).
		SetSeparator(' ')
	a.table.SetBorder(true).SetTitle(" Watching ")
	a.table.SetBorderColor(tcell.ColorDodgerBlue)
	a.table.SetTitleColor(tcell.ColorWhite)

	a.setTableHeaders()
	a.refreshTable()

	a.table.SetSelectedFunc(func(row, column int) {
		if row < 1 || row >= a.table.GetRowCount() {
			return
		}
		path := a.table.GetCell(row, 0).Text
		if path != "" {
			a.showWatchExplorer(path)
		}
	})

	// Main layout
	main := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(header, 8, 0, false).
		AddItem(a.table, 0, 1, true)

	// Pages for overlay support
	a.pages = tview.NewPages().
		AddPage("main", main, true, true)

	// Global key handler
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Only handle keys when main page is focused (no dialog open)
		if name, _ := a.pages.GetFrontPage(); name != "main" {
			return event
		}

		switch event.Rune() {
		case 'q':
			a.app.Stop()
			return nil
		case 'a':
			a.showFileBrowser("", func(path string) {
				a.showExpirationForm("New Watch", path, "")
			})
			return nil
		case 'e':
			path, exp := a.getSelectedWatch()
			if path == "" {
				return nil
			}
			a.showFileBrowser(path, func(selectedPath string) {
				a.showExpirationForm("Edit Watch", selectedPath, exp)
			})
			return nil
		case 'd':
			path, _ := a.getSelectedWatch()
			if path == "" {
				return nil
			}
			a.showUnwatchModal(path)
			return nil
		case 'x':
			a.showExpiredFiles()
			return nil
		case 's':
			a.showDaemonSettings()
			return nil
		}
		return event
	})

	a.app.SetRoot(a.pages, true)
}

func (a *App) setTableHeaders() {
	headers := []string{"PATH", "EXPIRATION", "STATUS"}
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetSelectable(false).
			SetTextColor(tcell.ColorWhite).
			SetAttributes(tcell.AttrBold).
			SetExpansion(1)
		if i == 1 {
			cell.SetExpansion(0).SetMaxWidth(20)
		}
		if i == 2 {
			cell.SetExpansion(0).SetMaxWidth(10)
		}
		a.table.SetCell(0, i, cell)
	}
}

func (a *App) refreshTable() {
	watches, err := service.ListWatches(a.db)
	if err != nil {
		return
	}

	// Clear data rows (keep header)
	rowCount := a.table.GetRowCount()
	for r := rowCount - 1; r >= 1; r-- {
		a.table.RemoveRow(r)
	}

	for i, w := range watches {
		row := i + 1 // offset for header

		expStr := "NULL"
		if w.DefaultExpiration.Valid {
			exp := w.DefaultExpiration.Int32
			expStr = formatDuration(exp)
		}

		a.table.SetCell(row, 0, tview.NewTableCell(w.AbsoluteFilePath).
			SetTextColor(tcell.ColorWhite).
			SetExpansion(1))
		a.table.SetCell(row, 1, tview.NewTableCell(expStr).
			SetTextColor(tcell.ColorLightGray).
			SetExpansion(0).SetMaxWidth(20))
		a.table.SetCell(row, 2, tview.NewTableCell("ACTIVE").
			SetTextColor(tcell.ColorGreen).
			SetExpansion(0).SetMaxWidth(10))
	}
}

func (a *App) refreshLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		a.app.QueueUpdateDraw(func() {
			a.refreshTable()
			a.refreshDaemonStatus()
		})
	}
}

func (a *App) refreshDaemonStatus() {
	hours := service.GetDaemonIntervalHours(a.db)
	intervalTag := fmt.Sprintf(" [gray](every %dh)[-]", hours)

	st := status.Read()
	if st == nil {
		a.daemonStatus.SetText("[gray]Daemon: never run[-]" + intervalTag)
		return
	}

	switch st.State {
	case status.StateRunning:
		elapsed := time.Since(st.StartedAt).Truncate(time.Second)
		a.daemonStatus.SetText(fmt.Sprintf(
			"[yellow]Daemon: running[-] [white](%s, %d files scanned)[-]%s",
			elapsed, st.FilesScanned, intervalTag,
		))
	case status.StateCompleted:
		ago := time.Since(st.FinishedAt).Truncate(time.Second)
		expiredText := ""
		if st.ExpiredFiles > 0 {
			expiredText = fmt.Sprintf(" [red]%d expired[-]", st.ExpiredFiles)
		}
		a.daemonStatus.SetText(fmt.Sprintf(
			"[green]Daemon: completed[-] [white]%s ago (%d files)[-]%s%s",
			humanAge(ago), st.FilesScanned, expiredText, intervalTag,
		))
	}
}

func (a *App) showDaemonSettings() {
	current := service.GetDaemonIntervalHours(a.db)

	options := make([]string, 12)
	for i := range options {
		h := i + 1
		if h == 1 {
			options[i] = "1 hour"
		} else {
			options[i] = fmt.Sprintf("%d hours", h)
		}
	}

	form := tview.NewForm()
	form.AddDropDown("Scan Interval", options, current-1, nil)
	form.AddButton("Save", func() {
		idx, _ := form.GetFormItemByLabel("Scan Interval").(*tview.DropDown).GetCurrentOption()
		hours := idx + 1
		service.SetDaemonIntervalHours(a.db, hours)
		a.refreshDaemonStatus()
		a.pages.RemovePage("dialog")
	})
	form.AddButton("Cancel", func() {
		a.pages.RemovePage("dialog")
	})

	form.SetBorder(true).
		SetTitle(" Daemon Settings ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorDodgerBlue).
		SetTitleColor(tcell.ColorWhite)
	form.SetButtonBackgroundColor(tcell.ColorDodgerBlue)
	form.SetFieldBackgroundColor(tcell.ColorDarkSlateGray)
	form.SetCancelFunc(func() {
		a.pages.RemovePage("dialog")
	})

	a.showCenteredDialog(form, 50, 13)
}

func (a *App) getSelectedWatch() (path, expiration string) {
	row, _ := a.table.GetSelection()
	if row < 1 || row >= a.table.GetRowCount() {
		return "", ""
	}
	path = a.table.GetCell(row, 0).Text
	expiration = a.table.GetCell(row, 1).Text
	if expiration == "NULL" {
		expiration = ""
	}
	return path, expiration
}

func (a *App) showExpirationForm(title, selectedPath, expValue string) {
	form := tview.NewForm()
	form.AddTextView("Path", selectedPath, 50, 1, false, false)
	form.AddInputField("Expiration (e.g. 30s, 5m, 24h, 7d, 3M)", expValue, 20, nil, nil)
	form.AddButton("OK", func() {
		expField := form.GetFormItemByLabel("Expiration (e.g. 30s, 5m, 24h, 7d, 3M)").(*tview.InputField)
		expStr := expField.GetText()

		var expiration *int32
		if matches := durationRegex.FindStringSubmatch(expStr); matches != nil {
			num, _ := strconv.Atoi(matches[1])
			multipliers := map[string]int{
				"s": 1,
				"m": 60,
				"h": 3600,
				"d": 86400,
				"M": 2592000, // 30 days
			}
			total := int32(num * multipliers[matches[2]])
			expiration = &total
		}

		service.UpsertWatch(a.db, selectedPath, expiration)
		a.refreshTable()
		a.pages.RemovePage("dialog")
	})
	form.AddButton("Cancel", func() {
		a.pages.RemovePage("dialog")
	})

	form.SetBorder(true).
		SetTitle(" " + title + " ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorDodgerBlue).
		SetTitleColor(tcell.ColorWhite)
	form.SetButtonBackgroundColor(tcell.ColorDodgerBlue)
	form.SetFieldBackgroundColor(tcell.ColorDarkSlateGray)
	form.SetCancelFunc(func() {
		a.pages.RemovePage("dialog")
	})

	a.showCenteredDialog(form, 60, 13)
}

func (a *App) showUnwatchModal(path string) {
	modal := tview.NewModal().
		SetText(fmt.Sprintf("Do you really want to unwatch\n%s folder?", path)).
		AddButtons([]string{"OK", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "OK" {
				service.DeleteWatch(a.db, path)
				a.refreshTable()
			}
			a.pages.RemovePage("dialog")
		})

	modal.SetBorderColor(tcell.ColorDodgerBlue)
	modal.SetBackgroundColor(tcell.ColorBlack)
	modal.SetTextColor(tcell.ColorWhite)
	modal.SetButtonBackgroundColor(tcell.ColorDarkSlateGray)
	modal.SetButtonActivatedStyle(tcell.StyleDefault.Background(tcell.ColorDodgerBlue).Foreground(tcell.ColorWhite))

	a.pages.AddPage("dialog", modal, true, true)
}

func (a *App) showCenteredDialog(primitive tview.Primitive, width, height int) {
	centered := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(primitive, height, 0, true).
				AddItem(nil, 0, 1, false),
			width, 0, true,
		).
		AddItem(nil, 0, 1, false)

	a.pages.AddPage("dialog", centered, true, true)
}

func formatDuration(seconds int32) string {
	months := seconds / 2592000
	days := (seconds % 2592000) / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if months > 0 {
		if days > 0 {
			return fmt.Sprintf("%dM %dd", months, days)
		}
		return fmt.Sprintf("%dM", months)
	}
	if days > 0 {
		if hours > 0 {
			return fmt.Sprintf("%dd %dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", secs)
}
