package main

import (
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemStats struct {
	Message     string
	OS          string
	CPUModel    string
	CPUCores    int
	CPUUsage    float64
	TotalMemory float64
	FreeMemory  float64
	Uptime      string
}

func getSystemStats() (SystemStats, error) {
	// Get CPU info
	cpuInfo, err := cpu.Info()
	if err != nil {
		return SystemStats{}, err
	}
	cpuUsage, err := cpu.Percent(0, false)
	if err != nil {
		return SystemStats{}, err
	}

	// Get memory info
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return SystemStats{}, err
	}

	// Get host info
	hostStat, err := host.Info()
	if err != nil {
		return SystemStats{}, err
	}

	stats := SystemStats{
		Message:     "EnChat is running",
		OS:          runtime.GOOS,
		CPUModel:    cpuInfo[0].ModelName,
		CPUCores:    runtime.NumCPU(),
		CPUUsage:    round(cpuUsage[0], 2),
		TotalMemory: bytesToGigabytes(vmStat.Total),
		FreeMemory:  bytesToGigabytes(vmStat.Free),
		Uptime:      formatUptime(hostStat.Uptime),
	}

	return stats, nil
}

func round(value float64, precision int) float64 {
	p := float64(precision)
	shift := float64(10 * p)
	return float64(int(value*shift)) / shift
}

func bytesToGigabytes(bytes uint64) float64 {
	return round(float64(bytes)/(1024*1024*1024), 2)
}

func formatUptime(seconds uint64) string {
	duration := time.Duration(seconds) * time.Second
	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	return fmt.Sprintf("%d days, %d hours, %d minutes", days, hours, minutes)
}

func handler(w http.ResponseWriter, r *http.Request) {
	stats, err := getSystemStats()
	if err != nil {
		http.Error(w, "Could not get system stats", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.New("index").Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>System Stats</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; padding: 20px; background-color: #f4f4f4; }
        h1 { color: #333; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th, td { padding: 12px; border: 1px solid #ddd; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <h1>{{.Message}}</h1>
    <table>
        <tr><th>Operating System</th><td>{{.OS}}</td></tr>
        <tr><th>CPU Model</th><td>{{.CPUModel}}</td></tr>
        <tr><th>CPU Cores</th><td>{{.CPUCores}}</td></tr>
        <tr><th>CPU Usage</th><td>{{.CPUUsage}}%</td></tr>
        <tr><th>Total Memory</th><td>{{.TotalMemory}} GB</td></tr>
        <tr><th>Free Memory</th><td>{{.FreeMemory}} GB</td></tr>
        <tr><th>Uptime</th><td>{{.Uptime}}</td></tr>
    </table>
</body>
</html>
`)
	if err != nil {
		http.Error(w, "Could not parse template", http.StatusInternalServerError)
		return
	}

	if err := tmpl.Execute(w, stats); err != nil {
		http.Error(w, "Could not execute template", http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/", handler)
	fmt.Println("Server is running at http://localhost:8080/")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Failed to start server: %v\n", err)
	}
}
