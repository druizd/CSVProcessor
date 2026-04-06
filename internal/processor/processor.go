package processor

import (
	"bufio"
	"csvprocessor/internal/config"
	"csvprocessor/internal/logger"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var fileNameRegex = regexp.MustCompile(`^log \((.*?)(?:--.*?)\)\s+(\d{4})_(\d{2})_(\d{2})_.*\.csv$`)

// openLocked implements an exclusive file lock for Windows
func openLocked(path string) (*os.File, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}

func ProcessFile(cfg *config.Config, filePath string) error {
	stat, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("error al leer estadisticas del archivo: %w", err)
	}

	// Wait 200ms from the last modification time
	timeSinceLastMod := time.Since(stat.ModTime())
	targetWait := time.Duration(cfg.DelayBeforeReadMs) * time.Millisecond
	if timeSinceLastMod < targetWait {
		sleepDuration := targetWait - timeSinceLastMod
		logger.Info("Esperando %v antes de procesar %s", sleepDuration, filepath.Base(filePath))
		time.Sleep(sleepDuration)
	}

	// Extract info from filename
	baseName := filepath.Base(filePath)
	matches := fileNameRegex.FindStringSubmatch(baseName)
	if len(matches) < 5 {
		return fmt.Errorf("el nombre del archivo no cumple el formato esperado: %s", baseName)
	}
	primaryIDFile := matches[1]
	receptionDate := fmt.Sprintf("%s-%s-%s", matches[2], matches[3], matches[4])

	// Open with exclusive lock
	file, err := openLocked(filePath)
	if err != nil {
		return fmt.Errorf("no se pudo abrir el archivo de forma exclusiva (quizas esta en uso): %w", err)
	}
	defer file.Close()

	// Read contents
	scanner := bufio.NewScanner(file)
	lineCount := 0

	var sqlBuffer strings.Builder
	// Pre-solicitamos memoria a la RAM para evitar fragmentaciones (ej: 2MB).
	sqlBuffer.Grow(2 * 1024 * 1024)

	sqlBuffer.WriteString("CREATE TABLE IF NOT EXISTS log_records (\n")
	sqlBuffer.WriteString("    primary_id    VARCHAR(50)  NOT NULL,\n")
	sqlBuffer.WriteString("    reception_date DATE        NOT NULL,\n")
	sqlBuffer.WriteString("    record_name   VARCHAR(50)  NOT NULL,\n")
	sqlBuffer.WriteString("    record_ts     TIMESTAMPTZ NOT NULL,\n")
	sqlBuffer.WriteString("    record_value  FLOAT\n")
	sqlBuffer.WriteString(");\n\n")
	// Convierte la tabla en hypertable de TimescaleDB particionada por record_ts.
	// if_not_exists => TRUE hace que sea idempotente (no falla si ya es hypertable).
	sqlBuffer.WriteString("SELECT create_hypertable('log_records', 'record_ts', if_not_exists => TRUE);\n\n")

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()

		// Ignorar encabezados y lineas vacias
		if lineCount <= 2 || len(line) == 0 {
			continue
		}

		// Extracción de parts[0] (antes de la primer coma)
		commaIdx1 := strings.IndexByte(line, ',')
		if commaIdx1 == -1 {
			continue
		}

		tagNameFull := line[:commaIdx1]
		dotIdx := strings.LastIndexByte(tagNameFull, '.')
		if dotIdx == -1 {
			continue
		}
		recordName := tagNameFull[dotIdx+1:]
		primaryIDRow := primaryIDFile

		// Extracción de parts[1] (entre primera y segunda coma)
		rest := line[commaIdx1+1:]
		commaIdx2 := strings.IndexByte(rest, ',')
		if commaIdx2 == -1 {
			continue
		}

		dateTimeFull := rest[:commaIdx2]
		spaceIdx := strings.IndexByte(dateTimeFull, ' ')
		if spaceIdx == -1 {
			continue
		}
		
		recordDate := dateTimeFull[:spaceIdx]
		recordTimeFull := dateTimeFull[spaceIdx+1:]
		
		var recordTime string
		if len(recordTimeFull) >= 5 {
			recordTime = recordTimeFull[:5]
		} else {
			recordTime = recordTimeFull
		}

		// Extracción de parts[2] (el valor), hasta la prox coma
		rest = rest[commaIdx2+1:]
		commaIdx3 := strings.IndexByte(rest, ',')
		var recordValue string
		if commaIdx3 == -1 {
			// Trim space solo en el último pedazo si hace falta
			recordValue = strings.TrimSpace(rest) 
		} else {
			recordValue = strings.TrimSpace(rest[:commaIdx3])
		}

		// Concatenación lineal (Zero allocations extra comparado a Sprintf)
		// record_ts combina record_date + record_time como TIMESTAMPTZ para la hypertable.
		sqlBuffer.WriteString("INSERT INTO log_records (primary_id, reception_date, record_name, record_ts, record_value) VALUES ('")
		sqlBuffer.WriteString(primaryIDRow)
		sqlBuffer.WriteString("', '")
		sqlBuffer.WriteString(receptionDate)
		sqlBuffer.WriteString("', '")
		sqlBuffer.WriteString(recordName)
		sqlBuffer.WriteString("', '")
		sqlBuffer.WriteString(recordDate)
		sqlBuffer.WriteString(" ")
		sqlBuffer.WriteString(recordTime)
		sqlBuffer.WriteString("', ")
		sqlBuffer.WriteString(recordValue)
		sqlBuffer.WriteString(");\n")
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error al leer el contenido del archivo: %w", err)
	}

	// Finished generating SQL.
	file.Close() // Explicitly close before moving

	// Generate target SQL path
	filenameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	sqlFileName := filenameWithoutExt + ".sql"
	sqlFilePath := filepath.Join(cfg.SqlLogDir, sqlFileName)

	err = os.WriteFile(sqlFilePath, []byte(sqlBuffer.String()), 0666)
	if err != nil {
		return fmt.Errorf("error escribiendo archivo SQL: %w", err)
	}
	
	// Move original CSV
	csvDestPath := filepath.Join(cfg.CsvLogDir, baseName)
	err = os.Rename(filePath, csvDestPath)
	if err != nil {
		return fmt.Errorf("error moviendo el archivo CSV a %s: %w. Asegurate que no haya colisiones de nombres.", csvDestPath, err)
	}

	return nil
}
