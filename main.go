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
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	hook "github.com/robotn/gohook"
	"github.com/seatedro/kawaiilogger/db"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

type DBConfig struct {
	Type     string
	URL      string
	FilePath string
}

type Metrics struct {
	Keypresses      int
	MouseClicks     int
	MouseDistanceIn float64
	MouseDistanceMi float64
	ScrollSteps     int
}

type TotalMetrics struct {
	TotalKeypresses      int
	TotalMouseClicks     int
	TotalMouseDistanceIn float64
	TotalMouseDistanceMi float64
	TotalScrollSteps     int
}

type Monitor struct {
	XPos     int
	YPos     int
	WidthPx  int
	HeightPx int
	WidthIn  int
	HeightIn int
	Ppi      int
}

var (
	dbQueries              *db.Queries
	_sqliteDb              *sql.DB
	metrics                *Metrics
	totalMetrics           *TotalMetrics
	logger                 *log.Logger
	logDir                 string
	lastMouseX, lastMouseY int
	monitors               []Monitor
	monitorsMutex          sync.RWMutex
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

func glfwInit() {
	if err := glfw.Init(); err != nil {
		panic(err)
	}
	defer glfw.Terminate()

	monitors = getMonitors()
}

func initializeDB() {
	if dbQueries != nil {
		return // Database already initialized
	}

	if err := setupDefaultSQLite(); err != nil {
		logger.Fatalf("failed to setup local sqlite: %v", err)
		return
	}

	config, err := loadConfig()
	if err != nil {
		logger.Fatalf("failed to load config: %e", err)
		return
	}

	var sqlDb *sql.DB
	switch config.Type {
	case "postgres":
		logger.Println("Connecting to postgres instance...")
		sqlDb, err = sql.Open("postgres", config.URL)
		dbQueries = db.New(sqlDb)
	case "sqlite":
		logger.Println("Connecting to sqlite instance...")
		if config.URL != "" {
			sqlDb, err = sql.Open("sqlite3", config.URL)
		} else {
			sqlDb, err = sql.Open("sqlite3", config.FilePath)
		}
		_sqliteDb = sqlDb
	case "libsql":
		logger.Println("Connecting to libsql instance...")
		sqlDb, err = sql.Open("libsql", config.URL)
		dbQueries = db.New(sqlDb)
	case "":
		logger.Println("Setting up default sqlite db...")
		setupDefaultSQLite()
		return
	default:
		logger.Fatalf("unsupported database type: %s", config.Type)
		return
	}

	if err != nil {
		logger.Fatalf("failed to open database: %e", err)
		return
	}

	if err = sqlDb.Ping(); err != nil {
		logger.Fatalf("failed to ping database: %e", err)
	}

}

func loadConfig() (DBConfig, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/kawaiilogger")

	err := viper.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config not found. ignore error
			logger.Println("No config file found... Using defaults.")
			return DBConfig{Type: ""}, nil
		} else {
			return DBConfig{}, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Load .env if it exists
	_ = godotenv.Load() // Ignores error if .env doesn't exist

	config := DBConfig{
		Type:     viper.GetString("database.type"),
		URL:      viper.GetString("database.url"),
		FilePath: viper.GetString("database.filepath"),
	}

	// Overriding with env vars if set
	if dbType := os.Getenv("KL_DB_TYPE"); dbType != "" {
		config.Type = dbType
	}
	if dbURL := os.Getenv("KL_DB_URL"); dbURL != "" {
		config.URL = dbURL
	}
	if dbFilePath := os.Getenv("KL_DB_FILEPATH"); dbFilePath != "" {
		config.FilePath = dbFilePath
	}

	return config, nil

}

func main() {
	initLogger()
	glfwInit()
	initializeDB()

	metrics = &Metrics{}
	totalMetrics = &TotalMetrics{}

	go collectMetrics()

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(getIcon())
	systray.SetTooltip("KawaiiLogger")

	mKeyPresses := systray.AddMenuItem("Keypresses: 0", "Number of keypresses")
	mMouseClicks := systray.AddMenuItem("Mouse Clicks: 0", "Number of mouse clicks")
	mMouseDistance := systray.AddMenuItem("Mouse Travel (in) 0 / (mi) 0", "Distance moved by mouse")
	mScrollSteps := systray.AddMenuItem("Scroll Steps: 0", "Number of scroll steps")

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
			mKeyPresses.SetTitle(fmt.Sprintf("Keypresses: %d", totalMetrics.TotalKeypresses))
			mMouseClicks.SetTitle(fmt.Sprintf("Mouse Clicks: %d", totalMetrics.TotalMouseClicks))
			mMouseDistance.SetTitle(fmt.Sprintf("Mouse Travel (in) %.2f / (mi) %.2f", totalMetrics.TotalMouseDistanceIn, totalMetrics.TotalMouseDistanceMi))
			mScrollSteps.SetTitle(fmt.Sprintf("Scroll Steps: %d", totalMetrics.TotalScrollSteps))
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
		totalMetrics.TotalKeypresses++
	})

	hook.Register(hook.MouseDown, nil, func(e hook.Event) {
		metrics.MouseClicks++
		totalMetrics.TotalMouseClicks++
	})

	// how the fuck do i track copy/paste?

	hook.Register(hook.MouseMove, nil, func(e hook.Event) {
		newX, newY := int(e.X), int(e.Y)
		distance := calculateMultiMonitorDistance(lastMouseX, lastMouseY, newX, newY)
		metrics.MouseDistanceIn += distance
		metrics.MouseDistanceMi += (distance / 63360)
		totalMetrics.TotalMouseDistanceIn += (distance)
		totalMetrics.TotalMouseDistanceMi += (distance / 63360)
		lastMouseX, lastMouseY = newX, newY
	})

	hook.Register(hook.MouseWheel, nil, func(e hook.Event) {
		distance := int(math.Abs(float64(e.Rotation)))
		metrics.ScrollSteps += distance
		totalMetrics.TotalScrollSteps += distance
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
	// We use the sqlite db here
	_, err := _sqliteDb.Exec(`
		INSERT INTO metrics (keypresses, mouse_clicks, mouse_distance_in, mouse_distance_mi, scroll_steps)
		VALUES (?, ?, ?, ?, ?)
	`, metrics.Keypresses, metrics.MouseClicks, metrics.MouseDistanceIn, metrics.MouseDistanceMi, metrics.ScrollSteps)

	if err != nil {
		logger.Printf("failed to save metrics: %v", err)
		return
	}

	if dbQueries != nil {
		_, err := dbQueries.CreateMetrics(context.Background(), db.CreateMetricsParams{
			Keypresses:      int32(metrics.Keypresses),
			MouseClicks:     int32(metrics.MouseClicks),
			MouseDistanceIn: metrics.MouseDistanceIn,
			MouseDistanceMi: metrics.MouseDistanceMi,
			ScrollSteps:     int32(metrics.ScrollSteps),
		})
		if err != nil {
			logger.Printf("Error saving metrics: %v", err)
		}
	}
	resetMetrics()
}

func resetMetrics() {
	metrics.Keypresses = 0
	metrics.MouseClicks = 0
	metrics.MouseDistanceIn = 0
	metrics.MouseDistanceMi = 0
	metrics.ScrollSteps = 0
}

func calculateDistance(x1, y1, x2, y2 int) float64 {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	return math.Sqrt(dx*dx + dy*dy)
}

func getMonitorSideCoordinates(x1, y1, x2, y2 int, m Monitor) (int, int) {
	// Get the coordinates for the side where the mouse leaves.
	if x2 < m.XPos {
		return m.XPos, y1
	} else if x2 >= m.XPos+m.WidthPx {
		return m.XPos + m.WidthPx - 1, y1
	} else if y2 < m.YPos {
		return x1, m.YPos
	} else if y2 >= m.YPos+m.HeightPx {
		return x1, m.YPos + m.HeightPx - 1
	}

	return x2, y2
}

func getMonitors() []Monitor {
	glfwMonitors := glfw.GetMonitors()
	monitors := make([]Monitor, len(glfwMonitors))

	for i, glfwMonitor := range glfwMonitors {
		videoMode := glfwMonitor.GetVideoMode()

		widthMM, heightMM := glfwMonitor.GetPhysicalSize()
		widthIn, heightIn := float64(widthMM)/25.4, float64(heightMM)/25.4
		xPos, yPos := glfwMonitor.GetPos()

		widthDpi := float64(videoMode.Width) / widthIn
		heightDpi := float64(videoMode.Height) / heightIn

		ppi := (widthDpi + heightDpi) / 2

		monitors[i] = Monitor{
			XPos:     xPos,
			YPos:     yPos,
			WidthPx:  videoMode.Width,
			HeightPx: videoMode.Height,
			WidthIn:  int(widthIn),
			HeightIn: int(heightIn),
			Ppi:      int(ppi),
		}
	}

	return monitors
}

func calculateMultiMonitorDistance(x1, y1, x2, y2 int) float64 {
	monitorsMutex.RLock()
	defer monitorsMutex.RUnlock()

	m1 := getMonitorForCoordinates(x1, y1)
	m2 := getMonitorForCoordinates(x2, y2)

	if m1 == m2 {
		return calculateDistance(x1, y1, x2, y2) / float64(m1.Ppi)
	}

	sx1, sy1 := getMonitorSideCoordinates(x1, y1, x2, y2, m1)
	d1 := calculateDistance(x1, y1, sx1, sy1) / float64(m1.Ppi)

	sx2, sy2 := getMonitorSideCoordinates(x1, y1, x2, y2, m2)
	d2 := calculateDistance(x1, y1, sx2, sy2) / float64(m1.Ppi)

	return d1 + d2
}

func getMonitorForCoordinates(x, y int) Monitor {
	for _, m := range monitors {
		if x >= m.XPos && x < (m.XPos+m.WidthPx) && y >= m.YPos && y < (m.YPos+m.HeightPx) {
			return m
		}
	}
	// Default to monitor 0
	return monitors[0]
}

func getIcon() []byte {
	iconPath := "./keyboard.ico"
	iconBytes, err := os.ReadFile(iconPath)
	if err != nil {
		log.Fatalf("Failed to read icon: %v", err)
	}

	return iconBytes
}

func setupDefaultSQLite() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home dir: %w", err)
	}

	dbDir := filepath.Join(homeDir, ".config", "kawaiilogger")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	dbPath := filepath.Join(dbDir, "kawaiilogger.db")
	sqlDb, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open default SQLite database: %w", err)
	}

	// Create tables if they don't exist
	_, err = sqlDb.Exec(`
		CREATE TABLE IF NOT EXISTS metrics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			keypresses INTEGER,
			mouse_clicks INTEGER,
			mouse_distance_in REAL,
			mouse_distance_mi REAL,
			scroll_steps INTEGER
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create default tables: %w", err)
	}

	logger.Println("Created default sqlite db...")
	_sqliteDb = sqlDb

	return nil

}
