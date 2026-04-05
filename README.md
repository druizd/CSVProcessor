# CSV to SQL Processor (Windows Service)

Deminio (Windows Service Nativo) desarrollado en Go para el procesamiento asíncrono y continuo de archivos CSV. Escanea un directorio de entrada, parsea contenidos de archivos bajo patrones estrictos usando manejo nativo de memoria (`strings.Builder`) y genera archivos SQL de inserción correspondientes. 

## Características

- **Servicio Nativo de Windows**: Utiliza de arquitectura cliente/servidor con el administrador local usando la librería O.S. `golang.org/x/sys/windows/svc`. Permite autoarranque con el sistema.
- **Rendimiento Industrial**: Cero-alocaciones intermitentes (`zero allocations`), sin dependencias de fragmentos genéricos como `fmt.Sprintf` o `strings.Split`.
- **Procesamiento de Alta Concurrencia**: Utiliza un pool de *Workers* limitados (2 por defecto).
- **Bloqueos Exclusivos (OS Locks)**: Implementa bloqueos a nivel de kernel en Windows (`syscall.O_EXCL`) para impedir lectura de medias escrituras.
- **Tiempos de Gracia y Apagado**: Capturas directas desde el Service Control Manager (SCM). Apagado limpio y seguro, terminando ciclos pendientes evitando bases de datos corruptas (`Graceful Shutdown`).

## Requisitos

- [Go](https://golang.org/dl/) 1.18 o superior.
- SO Windows Server / Windows 10/11 (para la gestión de servicios restrictivos).

## Instalación y Primer Uso

1. Clona el repositorio.
2. Construye el binario incluyendo las dependencias y agregando su icono nativo (*requiere go-winres si deseas reciclar el icono*):
```bash
go build -o csvprocessor.exe
```

La aplicación no se abrirá con un doble click convencional si quieres operarlo de un modo continuo, ya que requiere de un manejador de contexto administrativo.
Abre tu consola de **Windows (CMD o PowerShell) en Modo Administrador**, y usa los siguientes comandos CLI incorporados:

- **Instalar permanente en Windows:** `.\csvprocessor.exe install`
- **Iniciar demonio:** `.\csvprocessor.exe start`
- **Detener demonio:** `.\csvprocessor.exe stop`
- **Remover servicio:** `.\csvprocessor.exe remove`

> **Nota para debuggers:** Puedes usar `.\csvprocessor.exe debug` para realizar una prueba directamente corriendo los logs en tu consola a tiempo real y cortándolo con el comando clásico de `Ctrl+C`.

## Configuración 

El archivo `config.json` se encuentra siempre alojado junto al ejecutable (con anclaje inteligente). 

```json
{
  "input_dir": "./input",
  "sql_log_dir": "./sqllog",
  "csv_log_dir": "./csvlog",
  "logs_dir": "./logs",
  "max_agents": 2,
  "max_files_per_agent": 50,
  "delay_before_read_ms": 200
}
```

## Estructura del Código Fuente

```
/
├── config/        - Lógica de lectura con OS path binding forzado para System32.
├── logger/        - Centralización de logs locales indexados.
├── processor/     - Generación de byte scanning, CPU parsing y volcado.
├── worker/        - Orquestación persistente local del pool.
├── main.go        - SCM Handler, OS Hooks e inyectador de Comandos O.S (CLI).
```
