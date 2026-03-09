package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/joho/godotenv"

	"github.com/joaoalvarenga/dinha/internal/db"
	"github.com/joaoalvarenga/dinha/internal/service"
	"github.com/joaoalvarenga/dinha/internal/status"
)

var database *sql.DB

func main() {
	godotenv.Load()
	setupLogging()
	database = db.GetDB()
	systray.Run(onReady, onExit)
}

func setupLogging() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get home directory: %v", err)
	}
	logDir := filepath.Join(home, ".dinha")
	os.MkdirAll(logDir, 0755)

	logFile, err := os.OpenFile(filepath.Join(logDir, "log.out"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}
	log.SetOutput(logFile)
}

type scanResult struct {
	stopped      bool
	expiredCount int
}

func onReady() {
	icon := generateIcon()
	systray.SetTemplateIcon(icon, icon)
	systray.SetTooltip("Dinha - File Expiration Daemon")

	mStatus := systray.AddMenuItem("Starting...", "Current status")
	mStatus.Disable()
	systray.AddSeparator()
	mScanNow := systray.AddMenuItem("Scan Now", "Run scan immediately")
	mStopScan := systray.AddMenuItem("Stop Scan", "Cancel running scan")
	mStopScan.Hide()
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop dinha daemon")

	go daemonLoop(mStatus, mScanNow, mStopScan, mQuit)
}

func onExit() {
	if database != nil {
		database.Close()
	}
}

func daemonLoop(mStatus, mScanNow, mStopScan, mQuit *systray.MenuItem) {
	scanDone := make(chan scanResult, 1)
	var cancelScan context.CancelFunc
	scanning := false

	startScan := func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancelScan = cancel
		scanning = true
		mScanNow.Hide()
		mStopScan.Show()
		go func() {
			result := performScan(mStatus, ctx)
			scanDone <- result
		}()
	}

	var timerCh <-chan time.Time
	var timer *time.Timer
	var currentHours int

	scheduleNext := func() {
		currentHours = service.GetDaemonIntervalHours(database)
		interval := time.Duration(currentHours) * time.Hour
		timer = time.NewTimer(interval)
		timerCh = timer.C
		nextRun := time.Now().Add(interval)
		mStatus.SetTitle(fmt.Sprintf("Next scan: %s (every %dh)", nextRun.Format("15:04"), currentHours))
	}

	// Poll DB every 30s to detect interval changes from TUI
	configCheck := time.NewTicker(15 * time.Second)
	defer configCheck.Stop()

	startScan()

	for {
		select {
		case result := <-scanDone:
			scanning = false
			cancelScan = nil
			mStopScan.Hide()
			mScanNow.Show()

			if result.stopped {
				mStatus.SetTitle(fmt.Sprintf("Scan stopped at %s", time.Now().Format("15:04")))
				log.Println("Scan stopped by user")
			} else if result.expiredCount > 0 {
				mStatus.SetTitle(fmt.Sprintf("Last: %s (%d expired)", time.Now().Format("15:04"), result.expiredCount))
			} else {
				mStatus.SetTitle(fmt.Sprintf("Last scan: %s ✓", time.Now().Format("15:04")))
			}
			scheduleNext()

		case <-configCheck.C:
			if scanning {
				continue
			}
			newHours := service.GetDaemonIntervalHours(database)
			if newHours != currentHours {
				log.Printf("Interval changed: %dh → %dh", currentHours, newHours)
				if timer != nil {
					timer.Stop()
				}
				scheduleNext()
			}

		case <-timerCh:
			timerCh = nil
			startScan()

		case <-mScanNow.ClickedCh:
			if !scanning {
				if timer != nil {
					timer.Stop()
				}
				timerCh = nil
				startScan()
			}

		case <-mStopScan.ClickedCh:
			if scanning && cancelScan != nil {
				cancelScan()
			}

		case <-mQuit.ClickedCh:
			if scanning && cancelScan != nil {
				cancelScan()
			}
			if timer != nil {
				timer.Stop()
			}
			systray.Quit()
			return
		}
	}
}

func performScan(mStatus *systray.MenuItem, ctx context.Context) scanResult {
	mStatus.SetTitle("Scanning...")
	log.Println("Scan started")

	st := status.NewWriter()
	st.Start()

	stopped := runScan(ctx, database, st)
	if stopped {
		st.Finish(0)
		return scanResult{stopped: true}
	}

	expiredCount := checkExpired(database)
	st.Finish(expiredCount)

	return scanResult{expiredCount: expiredCount}
}

func runScan(ctx context.Context, db *sql.DB, st *status.DaemonStatus) bool {
	watches, err := service.ListWatches(db)
	if err != nil {
		log.Printf("error listing watches: %v", err)
		return false
	}

	for _, w := range watches {
		if ctx.Err() != nil {
			return true
		}

		root := expandPath(w.AbsoluteFilePath)

		var expSeconds *int32
		if w.DefaultExpiration.Valid {
			v := w.DefaultExpiration.Int32
			expSeconds = &v
		}

		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if ctx.Err() != nil {
				return filepath.SkipAll
			}
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}

			accessTime := info.ModTime() // fallback
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				accessTime = time.Unix(stat.Atimespec.Sec, stat.Atimespec.Nsec)
			}

			service.SyncFile(db, path, info.ModTime(), accessTime, expSeconds)
			st.IncrFiles(1)
			return nil
		})
		if err != nil {
			log.Printf("error scanning %s: %v", root, err)
		}
	}
	return ctx.Err() != nil
}

func checkExpired(db *sql.DB) int {
	expired, err := service.ListExpiredFiles(db, true)
	if err != nil {
		log.Printf("error listing expired files: %v", err)
		return 0
	}

	if len(expired) == 0 {
		log.Printf("No expired files found")
		return 0
	}

	log.Printf("Found %d expired files", len(expired))
	for _, f := range expired {
		log.Printf("  Expired: %s (expiration: %s)", f.AbsoluteFilePath, f.Expiration.Time.Format("2006-01-02 15:04"))
	}

	msg := fmt.Sprintf("%d arquivo(s) vencido(s) encontrado(s)", len(expired))
	notify("Dinha", msg)

	return len(expired)
}

func notify(title, message string) {
	// Try terminal-notifier first (supports click-to-open and priority)
	if path, err := exec.LookPath("terminal-notifier"); err == nil {
		cmd := exec.Command(path,
			"-title", title,
			"-message", message,
			"-sound", "default",
			"-execute", "/usr/local/bin/dinha",
			"-ignoreDnD",
		)
		if err := cmd.Run(); err != nil {
			log.Printf("terminal-notifier error: %v", err)
		}
		return
	}

	// Fallback to osascript with sound
	script := fmt.Sprintf(`display notification %q with title %q sound name "Default"`, message, title)
	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		log.Printf("error sending notification: %v", err)
	}
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}
	return path
}

// generateIcon creates a 22x22 template icon (document shape) for the macOS menu bar.
func generateIcon() []byte {
	const size = 22
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	black := color.RGBA{0, 0, 0, 255}
	foldSize := 5

	// Document body
	left, top, right, bottom := 5, 2, 16, 19
	for y := top; y <= bottom; y++ {
		for x := left; x <= right; x++ {
			// Cut the top-right corner for the fold
			if y < top+foldSize && x > right-foldSize {
				dx := x - (right - foldSize)
				dy := (top + foldSize) - y
				if dx > dy {
					continue
				}
			}
			img.Set(x, y, black)
		}
	}

	// Fold diagonal
	for i := 0; i <= foldSize; i++ {
		fx := right - foldSize + i
		fy := top + i
		img.Set(fx, fy, black)
		// Small triangle fill for the fold flap
		for y := fy; y <= top+foldSize; y++ {
			dist := float64((y - fy) + (fx - (right - foldSize)))
			if dist <= float64(foldSize) {
				img.Set(fx, y, black)
			}
		}
	}

	// Horizontal lines inside the document (text lines)
	lineColor := color.RGBA{0, 0, 0, 80}
	for _, ly := range []int{10, 13, 16} {
		for x := 8; x <= 14; x++ {
			// Anti-alias edges
			if x == 8 || x == 14 {
				img.Set(x, ly, color.RGBA{0, 0, 0, 40})
			} else {
				img.Set(x, ly, lineColor)
			}
		}
	}

	// Smooth edges with basic anti-aliasing on the outline
	for y := top; y <= bottom; y++ {
		for x := left; x <= right; x++ {
			r, _, _, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			// Check if this is an edge pixel
			neighbors := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					_, _, _, na := img.At(x+dx, y+dy).RGBA()
					if na > 0 {
						neighbors++
					}
				}
			}
			_ = r
			// Soften corner pixels
			if neighbors <= 3 {
				c := img.RGBAAt(x, y)
				c.A = uint8(math.Min(float64(c.A), 180))
				img.SetRGBA(x, y, c)
			}
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
