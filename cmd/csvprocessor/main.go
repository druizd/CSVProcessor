package main

import (
	"csvprocessor/internal/api"
	"csvprocessor/internal/config"
	"csvprocessor/internal/logger"
	"csvprocessor/internal/worker"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

var svcName = "CSVProcessorService"

type myService struct{}

func (m *myService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	
	// Initialization Logic
	cfg, err := config.LoadConfig("config.json")
	if err != nil {
		// Log directly to windows eventviewer if possible, otherwise fail
		return false, 1
	}

	err = logger.InitLogger(cfg.LogsDir)
	if err != nil {
		return false, 2
	}
	defer logger.CloseLogger()

	logger.Event("Servicio CSV Processor iniciado en modo nativo de Windows (SCM).")
	
	// Arranque de API Remota
	if cfg.ApiPort > 0 {
		go api.StartServer(cfg.ApiPort)
	}
	
	fileChan := make(chan string, 100)
	var wg sync.WaitGroup
	doneChan := make(chan bool)

	worker.StartPool(cfg, fileChan, &wg)

	go func() {
		queuedFiles := make(map[string]bool)
		for {
			select {
			case <-doneChan:
				return
			default:
				currentFiles := make(map[string]bool)
				files, err := os.ReadDir(cfg.InputDir)
				if err == nil {
					for _, file := range files {
						if !file.IsDir() && strings.HasSuffix(strings.ToLower(file.Name()), ".csv") {
							fullPath := filepath.Join(cfg.InputDir, file.Name())
							currentFiles[fullPath] = true
							if !queuedFiles[fullPath] {
								queuedFiles[fullPath] = true
								fileChan <- fullPath
							}
						}
					}
				} else {
					logger.Error("Error leyendo directorio de entrada %s: %v", cfg.InputDir, err)
				}
				
				for path := range queuedFiles {
					if !currentFiles[path] {
						delete(queuedFiles, path)
					}
				}
				time.Sleep(1 * time.Second)
			}
		}
	}()

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		c := <-r
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			logger.Event("Recibida señal de Stop/Shutdown desde Administrador de Servicios. Apagando...")
			break loop
		default:
			logger.Error("Comando de servicio inesperado recibido de SCM: %v", c)
		}
	}
	changes <- svc.Status{State: svc.StopPending}

	close(doneChan)
	close(fileChan)
	logger.Info("Esperando a que los agentes terminen sus tareas actuales...")
	wg.Wait()
	logger.Event("Servicio CSV Processor detenido correctamente.")
	
	return false, 0
}

func main() {
	inService, err := svc.IsWindowsService()
	if err != nil {
		fmt.Printf("Error determinando sistema anfitrion: %v\n", err)
		return
	}

	if inService {
		runService(svcName, false)
		return
	}

	if len(os.Args) < 2 {
		fmt.Println("Uso: csvprocessor.exe <comando>")
		fmt.Println("Opciones disponibles:")
		fmt.Println("  install - Instala el programa en services.msc para arranque autom\u00E1tico")
		fmt.Println("  remove  - Elimina el servicio de Windows")
		fmt.Println("  start   - Inicia el demonio del servicio")
		fmt.Println("  stop    - Detiene el demonio del servicio")
		fmt.Println("  debug   - Ejecuta directo en consola para pruebas (usa Ctrl+C para salir)")
		return
	}

	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "debug":
		runService(svcName, true)
	case "install":
		err = installService(svcName, "CSV Processor nativo. Levanta agentes goroutines para transformar ficheros logs a archivos de insersion SQL.")
	case "remove":
		err = removeService(svcName)
	case "start":
		err = startService(svcName)
	case "stop":
		err = controlService(svcName, svc.Stop, svc.Stopped)
	default:
		fmt.Printf("Comando inv\u00E1lido '%s'. Usa csvprocessor.exe sin parametros para ver ayuda.\n", cmd)
		return
	}
	
	if err != nil {
		fmt.Printf("Hubo un fallo ejecutando el comando '%s': %v\n", cmd, err)
		fmt.Println("NOTA: Los comandos config necesitan consola abierta como Administrador.")
	} else if cmd != "debug" {
		fmt.Printf("Comando %s ejecutado exitosamente.\n", cmd)
	}
}

func runService(name string, isDebug bool) {
	var err error
	if isDebug {
		err = debug.Run(name, &myService{})
	} else {
		err = svc.Run(name, &myService{})
	}
	if err != nil {
		fmt.Printf("Servicio caido o cerrado: %v\n", err)
	}
}

// Helpers para encontrar ruta de instalacion exacta en vez de /Windows/System32
func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s es un directorio", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("%s es un directorio", p)
		}
	}
	return "", err
}

func installService(name, desc string) error {
	exepath, err := exePath()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("El demonio %s ya se encontraba instalado", name)
	}
	s, err = m.CreateService(name, exepath, mgr.Config{DisplayName: name, Description: desc, StartType: mgr.StartAutomatic})
	if err != nil {
		return err
	}
	defer s.Close()
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("SetupEventLogSource() fallo: %s", err)
	}
	return nil
}

func removeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("El servicio %s no existe o ya fue eliminado", name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	return nil
}

func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("No se logro comunicar con el servicio. Quiza no se instalo: %v", err)
	}
	defer s.Close()
	return s.Start()
}

func controlService(name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("No se obtuvo conexion RPC del servicio: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("El servicio declino el cambio de estado %s: %v", name, err)
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("Timeout: El servicio tarda mucho en responder a la orden")
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("Error consultando la cola de apagado: %v", err)
		}
	}
	return nil
}
