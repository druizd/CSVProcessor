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
	startTime                time.Time
	filesProcessed           uint64
	filesFailed              uint64
	totalProcessingTimeMs    uint64
	maxProcessingTimeMs      uint64
)

func init() {
	startTime = time.Now()
}

// RecordMetrics guarda de forma segura los atributos de tiempo
func RecordMetrics(success bool, durationMs uint64) {
	if success {
		atomic.AddUint64(&filesProcessed, 1)
		atomic.AddUint64(&totalProcessingTimeMs, durationMs)
		
		// Update Max safely
		for {
			currentMax := atomic.LoadUint64(&maxProcessingTimeMs)
			if durationMs <= currentMax {
				break
			}
			if atomic.CompareAndSwapUint64(&maxProcessingTimeMs, currentMax, durationMs) {
				break
			}
		}
	} else {
		atomic.AddUint64(&filesFailed, 1)
	}
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	processed := atomic.LoadUint64(&filesProcessed)
	failed := atomic.LoadUint64(&filesFailed)
	totalTimeMs := atomic.LoadUint64(&totalProcessingTimeMs)
	maxTimeMs := atomic.LoadUint64(&maxProcessingTimeMs)

	var avgTimeMs uint64 = 0
	if processed > 0 {
		avgTimeMs = totalTimeMs / processed
	}

	// Redondeo del Uptime para eliminar milisegundos/microsegundos
	uptime := time.Since(startTime).Round(time.Second).String()

	response := map[string]interface{}{
		"status":               "UP",
		"uptime":               uptime,
		"archivos_procesados":  processed,
		"archivos_fallidos":    failed,
		"promedio_proceso_ms":  avgTimeMs,
		"tiempo_maximo_ms":     maxTimeMs,
	}
	json.NewEncoder(w).Encode(response)
}

func StartServer(port int) {
	mux := http.NewServeMux()
	
	// Consolidado: Ambas antiguas rutas devolverán ahora la vista completa
	mux.HandleFunc("/health", statusHandler)
	mux.HandleFunc("/metrics", statusHandler)

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
