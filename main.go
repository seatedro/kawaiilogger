package main

import (
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
)

type Metrics struct {
	Keypresses     int
	MouseClicks    int
	IdleTime       time.Duration
	CopyPasteCount int
}

var db *sql.DB
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

	db, err = sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Fatal("Error connecting to database:", err)
	}
	defer db.Close()

	createDbSchema()

	metrics = &Metrics{}

	go collectMetrics()

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(getIcon())
	systray.SetTitle("kl")
	systray.SetTooltip("KawaiiLogger")

	mKeyPresses := systray.AddMenuItem("Keypresses: 0", "Number of keypresses")
	mMouseClicks := systray.AddMenuItem("Mouse Clicks: 0", "Number of mouse clicks")
	mCopyPaste := systray.AddMenuItem("Copy/Paste: 0", "Number of copy/paste operations")

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
			mCopyPaste.SetTitle(fmt.Sprintf("Copy/Paste: %d", metrics.CopyPasteCount))
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

	hook.Register(hook.KeyDown, nil, func(e hook.Event) {
		if e.Rawcode == 67 && e.Keycode == 0 { // 'C' key
			metrics.CopyPasteCount++
		} else if e.Rawcode == 86 && e.Keycode == 0 { // 'V' key
			metrics.CopyPasteCount++
		}
	})

	go func() {
		for {
			time.Sleep(time.Second * 30)
			saveMetrics()
		}
	}()

	s := hook.Start()
	<-hook.Process(s)
}

func createDbSchema() {

	_, err := db.Exec(`CREATE DATABASE kawaiilogger`)
	if err != nil {
		logger.Printf("Error creating the database: %v", err)
	} else {
		logger.Printf("Database created successfully")
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS metrics (
		keypresses NUMERIC,
		mouse_clicks NUMERIC,
		idle_time REAL,
		copy_paste_count NUMERIC,
		timestamp TIMESTAMP
	)`)
	if err != nil {
		logger.Printf("Error creating the table: %v", err)
	} else {
		logger.Printf("Table created successfully")
	}
}

func saveMetrics() {
	_, err := db.Exec(`INSERT INTO metrics 
		(keypresses, mouse_clicks, idle_time, copy_paste_count, timestamp) 
		VALUES ($1, $2, $3, $4, $5)`,
		metrics.Keypresses, metrics.MouseClicks, metrics.IdleTime.Seconds(), metrics.CopyPasteCount, time.Now())
	if err != nil {
		logger.Printf("Error saving metrics: %v", err)
	} else {
		logger.Println("Metrics saved successfully")
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
