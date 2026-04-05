package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"csvprocessor/internal/logger"
)

var (
	startTime      time.Time
	filesProcessed uint64
)

func init() {
	startTime = time.Now()
}

// IncrementProcessed safely increments the files processed counter metric
func IncrementProcessed() {
	atomic.AddUint64(&filesProcessed, 1)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	uptime := time.Since(startTime).String()
	response := map[string]string{
		"status": "UP",
		"uptime": uptime,
	}
	json.NewEncoder(w).Encode(response)
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"archivos_procesados": atomic.LoadUint64(&filesProcessed),
	}
	json.NewEncoder(w).Encode(response)
}

func StartServer(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/metrics", metricsHandler)

	addr := fmt.Sprintf("0.0.0.0:%d", port) // 0.0.0.0 para acceso externo
	logger.Event("Iniciando API de Auditoría Remota en %s", addr)
	
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Fallo critico en API remota: %v", err)
		}
	}()
}
