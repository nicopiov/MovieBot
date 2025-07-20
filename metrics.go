// metrics.go
package main

import (
	"encoding/json"
	"fmt"
	"github.com/shirou/gopsutil/v3/process"
	"log/slog"
	"net/http"
	"os"
	"time"
)

var startTime = time.Now()

type SystemStats struct {
	BotID         string    `json:"bot_id"`
	Timestamp     time.Time `json:"timestamp"`
	Uptime        string    `json:"uptime"`
	UptimeSeconds float64   `json:"uptime_seconds"`
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryMB      float64   `json:"memory_mb"`
}

func StartMonitoringServer(port string) {
	// Setup HTTP handlers with context support
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		stats := systemStats()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			slog.Error("Failed to encode metrics: " + err.Error())
		}
	})

	slog.Info("Monitoring server starting on port " + port)
	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			slog.Error("Monitoring server failed to start: " + err.Error())
		}
	}()
}

func systemStats() SystemStats {
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return SystemStats{Timestamp: time.Now()}
	}

	cpuPercent, _ := p.CPUPercent()
	memInfo, _ := p.MemoryInfo()
	var memoryMB float64
	if memInfo != nil {
		memoryMB = float64(memInfo.RSS) / 1024 / 1024
	}

	uptime := time.Since(startTime)

	return SystemStats{
		BotID:         os.Getenv("BOT_ID"),
		Timestamp:     time.Now(),
		Uptime:        formatDuration(uptime),
		UptimeSeconds: uptime.Seconds(),
		CPUPercent:    cpuPercent,
		MemoryMB:      memoryMB,
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	return fmt.Sprintf("%dh %dm", h, m)
}
