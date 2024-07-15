package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/getlantern/systray"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/robotn/gohook"
	"github.com/seatedro/kawaiilogger/db"
)

type Metrics struct {
	Keypresses  int
	MouseClicks int
}

var dbQueries *db.Queries
var metrics *Metrics
var logger *log.Logger
var logDir string

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
	// systray.SetTitle("kl")
	systray.SetTooltip("KawaiiLogger")

	mKeyPresses := systray.AddMenuItem("Keypresses: 0", "Number of keypresses")
	mMouseClicks := systray.AddMenuItem("Mouse Clicks: 0", "Number of mouse clicks")

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
		Keypresses:  int32(metrics.Keypresses),
		MouseClicks: int32(metrics.MouseClicks),
	})
	if err != nil {
		logger.Printf("Error saving metrics: %v", err)
	} else {
		metrics.Keypresses = 0
		metrics.MouseClicks = 0
	}
}

func getIcon() []byte {
	iconPath := "./keyboard.ico"
	iconBytes, err := os.ReadFile(iconPath)
	if err != nil {
		log.Fatalf("Failed to read icon: %v", err)
	}

	return iconBytes
}
