# csvprocessor — Contexto del Repositorio

## ¿Qué es?

Servicio **Windows** (ejecutable nativo `.exe`) responsable de la **primera etapa del pipeline**:  
Convierte archivos `.csv` de logs de trading en sentencias SQL compatibles con TimescaleDB y los publica en RabbitMQ para su ejecución remota.

Corre como un **Servicio Nativo de Windows** (`services.msc`) con arranque automático, arquitectura de Worker Pool y una API HTTP interna para monitoreo remoto.

## Posición en el sistema

```
[Fuente de CSVs] ──.csv─→ [csvprocessor (Win)] ──SQL─→ [RabbitMQ (nube)] ──→ [csvconsumer (Linux)] ──→ [TimescaleDB]
                              ↑
                     Escaneo cada 1s del InputDir
```

## Stack técnico

| Componente | Detalle |
|------------|---------|
| Lenguaje | Go (Windows-only: `GOOS=windows`) |
| Mensajería | Sin RabbitMQ directo — genera SQL en disco, el shipper lo envía |
| Windows Service | `golang.org/x/sys/windows/svc` |
| Logging | Sistema propio (`internal/logger`) con rotación de archivos |
| API interna | `net/http` estándar de Go |

## Estructura del proyecto

```
csvprocessor/
├── cmd/
│   └── csvprocessor/
│       └── main.go          # Punto de entrada: lógica de servicio Windows, comandos CLI
├── internal/
│   ├── api/                 # Servidor HTTP de métricas y health check
│   ├── config/              # Carga y validación de config.json
│   ├── logger/              # Logger con rotación de archivos de log
│   ├── processor/           # Lógica de conversión CSV → SQL (TimescaleDB hypertable)
│   └── worker/              # Worker Pool con goroutines y bloqueo de archivos OS
├── config.json              # Configuración de producción
├── winres/                  # Recursos del ejecutable Windows (icono, versión, etc.)
└── csvprocessor.exe         # Binario compilado listo para despliegue
```

## Flujo de procesamiento

1. El **Scanner** escanea `InputDir` cada segundo buscando archivos `.csv`
2. Los archivos nuevos se envían a un **channel de jobs** (buffer: 100 tareas)
3. Los **Workers** (configurable, típicamente 2) toman archivos del channel
4. Cada worker usa **bloqueo a nivel OS** (rename a `.processing`) para evitar doble procesamiento
5. El **Processor** convierte el CSV a SQL (INSERT INTO TimescaleDB hypertable)
6. El archivo procesado se mueve a `DoneDir`; en caso de error, a `ErrorDir`

## Comandos CLI

```powershell
# Ejecutar como Administrador:
csvprocessor.exe install   # Instala en services.msc con arranque automático
csvprocessor.exe start     # Inicia el servicio
csvprocessor.exe stop      # Detiene el servicio
csvprocessor.exe remove    # Desinstala el servicio
csvprocessor.exe debug     # Ejecuta en consola interactiva (para pruebas con Ctrl+C)
```

## Configuración (`config.json`)

```json
{
  "input_dir":    "ruta donde caen los CSV",
  "done_dir":     "ruta para CSV procesados exitosamente",
  "error_dir":    "ruta para CSV con errores",
  "logs_dir":     "ruta donde se escriben los logs del servicio",
  "worker_count": 2,
  "api_port":     8080
}
```

## API de Métricas

Cuando `api_port > 0`, expone un servidor HTTP (por defecto en el puerto 8080) para monitoreo remoto del estado del servicio, archivos procesados y errores activos.

## Módulos internos clave

| Paquete | Responsabilidad |
|---------|-----------------|
| `internal/config` | Deserializa `config.json` y aplica defaults |
| `internal/logger` | Logger de archivos con niveles Info/Error/Event |
| `internal/processor` | Parsea CSV y genera SQL `INSERT` para TimescaleDB hypertable |
| `internal/worker` | Worker Pool: toma archivos del channel, llama al processor, mueve archivos |
| `internal/api` | Servidor HTTP con endpoints de métricas y health |

## Notas de despliegue

- Requiere ejecución como **Administrador** para instalar/desinstalar el servicio
- El `.exe` se registra en el SCM (Service Control Manager) de Windows apuntando a su ruta absoluta
- Integra con el **Event Viewer de Windows** (`eventlog`) para logs persistentes del sistema
- El binario `csvprocessor.exe` y `config.json` deben estar en el **mismo directorio**

## Repositorios relacionados

| Repositorio | Relación |
|-------------|----------|
| `csvshipper-win` | Servicio hermano: toma los SQL generados por este y los envía a RabbitMQ |
| `csvconsumer` | Ejecuta en Linux los SQL enviados a RabbitMQ |
| `db-infra` | Provee la base de datos TimescaleDB destino |
