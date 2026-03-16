package common

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var (
	logger *log.Logger
	logFile *os.File
	auditLogger *log.Logger
	auditLogFile *os.File
)

func InitLogger(path string, maxSizeMB int) {
	auditPath := ""
	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, ".local", "state", "kaf", "kaf.log")
			auditPath = filepath.Join(home, ".local", "state", "kaf", "audit.log")
			os.MkdirAll(filepath.Dir(path), 0700)
		} else {
			path = fmt.Sprintf("/tmp/kaf-%d.log", os.Getuid())
			auditPath = fmt.Sprintf("/tmp/kaf-audit-%d.log", os.Getuid())
		}
	} else {
		auditPath = filepath.Join(filepath.Dir(path), "audit.log")
	}

	if maxSizeMB <= 0 {
		maxSizeMB = 10
	}
	
	// Basic rotation: if file > maxSizeMB, rotate it to .old
	if info, err := os.Stat(path); err == nil {
		if info.Size() > int64(maxSizeMB)*1024*1024 {
			oldPath := path + ".old"
			os.Remove(oldPath)
			os.Rename(path, oldPath)
		}
	}
	if info, err := os.Stat(auditPath); err == nil {
		if info.Size() > int64(maxSizeMB)*1024*1024 {
			oldPath := auditPath + ".old"
			os.Remove(oldPath)
			os.Rename(auditPath, oldPath)
		}
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		// Fallback to stderr if path is unwritable
		logger = log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile)
	} else {
		logFile = f
		logger = log.New(f, "", log.LstdFlags|log.Lshortfile)
	}
	
	af, err := os.OpenFile(auditPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err == nil {
		auditLogFile = af
		auditLogger = log.New(af, "AUDIT: ", log.LstdFlags)
	}

	Log("--- KAF SESSION STARTED (Path: %s, MaxSize: %dMB) ---", path, maxSizeMB)
}

func Log(format string, v ...interface{}) {
	if logger != nil {
		logger.Output(2, fmt.Sprintf(format, v...))
	}
}

func AuditLog(action, details string) {
	if auditLogger != nil {
		auditLogger.Printf("ACTION=%s DETAILS=%s", action, details)
	}
}

func CloseLogger() {
	if logFile != nil {
		logFile.Close()
	}
	if auditLogFile != nil {
		auditLogFile.Close()
	}
}
