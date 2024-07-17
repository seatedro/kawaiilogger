package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/getlantern/systray"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	hook "github.com/robotn/gohook"
	"github.com/seatedro/kawaiilogger/db"
)

type Metrics struct {
	Keypresses     int
	MouseClicks    int
	MouseDistance  float64
	ScrollDistance float64
}

var (
	dbQueries              *db.Queries
	metrics                *Metrics
	logger                 *log.Logger
	logDir                 string
	lastMouseX, lastMouseY int16
)

func initLogger() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Error getting user home directory:", err)
	}
	switch os := runtime.GOOS; os {
	case "darwin":
		// macOS
		logDir = filepath.Join(homeDir, ".config", "kawaiilogger")
	case "windows":
		// Windows
		logDir = "C:\\ProgramData\\kawaiilogger\\Logs\\"
	case "linux":
		// Linux
		logDir = filepath.Join(homeDir, ".config", "kawaiilogger")
	default:
		logDir = "./"
	}

	err = os.MkdirAll(logDir, 0755)
	if err != nil {
		log.Fatal("Error creating log directory:", err)
	}

	logFile := filepath.Join(logDir, "kawaiilogger.log")
	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("Error opening log file:", err)
	}

	logger = log.New(file, "", log.Ldate|log.Ltime|log.Lshortfile)
	logger.Println("kawaiilogger started")
}

func main() {
	initLogger()

	logger.Printf("Current working directory: %s\n", os.Getenv("PWD"))
	err := godotenv.Load()
	if err != nil {
		logger.Fatal("Error loading .env file:", err)
	}

	sqlDb, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Fatal("Error connecting to database:", err)
	}
	defer sqlDb.Close()

	dbQueries = db.New(sqlDb)

	metrics = &Metrics{}

	go collectMetrics()

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(getIcon())
	systray.SetTooltip("KawaiiLogger")

	mKeyPresses := systray.AddMenuItem("Keypresses: 0", "Number of keypresses")
	mMouseClicks := systray.AddMenuItem("Mouse Clicks: 0", "Number of mouse clicks")
	mMouseDistance := systray.AddMenuItem("Mouse Travel Distance: 0", "Distance moved by mouse")
	mScrollDistance := systray.AddMenuItem("ScrollWheel Travel Distance: 0", "Distance moved by scrollwheel")

	systray.AddSeparator()
	mOpenLog := systray.AddMenuItem("Open Log File", "Open the log file")
	mQuit := systray.AddMenuItem("Quit", "Quit the application")

	go func() {
		for {
			select {
			case <-mOpenLog.ClickedCh:
				openLogFile()
			case <-mQuit.ClickedCh:
				logger.Println("kawaiilogger shutting down")
				systray.Quit()
				return
			}
		}
	}()

	go func() {
		for {
			time.Sleep(time.Second)
			mKeyPresses.SetTitle(fmt.Sprintf("Keypresses: %d", metrics.Keypresses))
			mMouseClicks.SetTitle(fmt.Sprintf("Mouse Clicks: %d", metrics.MouseClicks))
			mMouseDistance.SetTitle(fmt.Sprintf("Mouse Travel: %.2f", metrics.MouseDistance))
			mScrollDistance.SetTitle(fmt.Sprintf("ScrollWheel Travel: %.2f", metrics.ScrollDistance))
		}
	}()
}

func onExit() {
	// Cleanup
}

func openLogFile() {
	logFile := filepath.Join(logDir, "kawaiilogger.log")
	var command string
	switch os := runtime.GOOS; os {
	case "darwin":
		// macOS
		command = "open"
	case "windows":
		// Windows
		command = "start"
	case "linux":
		// Linux
		command = "open"
	default:
		command = "open"
	}
	cmd := exec.Command(command, logFile)
	err := cmd.Start()
	if err != nil {
		logger.Printf("Error opening log file: %v", err)
	}
}

func collectMetrics() {
	hook.Register(hook.KeyDown, nil, func(e hook.Event) {
		metrics.Keypresses++
	})

	hook.Register(hook.MouseDown, nil, func(e hook.Event) {
		metrics.MouseClicks++
	})

	// how the fuck do i track copy/paste?

	hook.Register(hook.MouseMove, nil, func(e hook.Event) {
		newX, newY := e.X, e.Y
		distance := calculateDistance(lastMouseX, lastMouseY, newX, newY)
		metrics.MouseDistance += distance
		lastMouseX, lastMouseY = newX, newY
	})

	hook.Register(hook.MouseWheel, nil, func(e hook.Event) {
		metrics.ScrollDistance += float64(e.Rotation)
	})

	go func() {
		for {
			time.Sleep(time.Second * 60)
			saveMetrics()
		}
	}()

	s := hook.Start()
	<-hook.Process(s)
}

func saveMetrics() {
	_, err := dbQueries.CreateMetrics(context.Background(), db.CreateMetricsParams{
		Keypresses:     int32(metrics.Keypresses),
		MouseClicks:    int32(metrics.MouseClicks),
		MouseDistance:  metrics.MouseDistance,
		ScrollDistance: metrics.ScrollDistance,
	})
	if err != nil {
		logger.Printf("Error saving metrics: %v", err)
	} else {
		metrics.Keypresses = 0
		metrics.MouseClicks = 0
		metrics.MouseDistance = 0
		metrics.ScrollDistance = 0
	}
}

func calculateDistance(x1, y1, x2, y2 int16) float64 {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	return math.Sqrt(dx*dx + dy*dy)
}

func getIcon() []byte {
	iconPath := "./keyboard.ico"
	iconBytes, err := os.ReadFile(iconPath)
	if err != nil {
		log.Fatalf("Failed to read icon: %v", err)
	}

	return iconBytes
}
