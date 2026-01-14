package discovery

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/MinaroShikuchi/nginx-ui/nginx"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

const AppManifestDir = "/opt/nginx-manager/apps"

type AppManifest struct {
	Domain   string `yaml:"domain"`
	Protocol string `yaml:"protocol"`
	Hostname string `yaml:"hostname"`
	Port     int    `yaml:"port"`
}

type Watcher struct {
	Manager         *nginx.Manager
	AppsDir         string
	NginxListenPort int
}

func NewWatcher(mgr *nginx.Manager, appsDir string, nginxListenPort int) *Watcher {
	if appsDir == "" {
		appsDir = AppManifestDir
	}
	if nginxListenPort == 0 {
		nginxListenPort = 80
	}
	// Ensure directory exists
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		log.Printf("Warning: Failed to create apps dir %s: %v", appsDir, err)
	}

	return &Watcher{
		Manager:         mgr,
		AppsDir:         appsDir,
		NginxListenPort: nginxListenPort,
	}
}

func (w *Watcher) Start() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					log.Println("modified file:", event.Name)
					w.handleFileChange(event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(w.AppsDir)
	if err != nil {
		log.Printf("Failed to watch %s: %v", w.AppsDir, err)
		// Don't crash if we can't watch strictly, but essentially this feature fails.
	} else {
		log.Printf("Watching %s for new apps...", w.AppsDir)
	}
	<-done
}

func (w *Watcher) handleFileChange(path string) {
	if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
		return
	}

	// 1. Read YAML
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Failed to read %s: %v", path, err)
		return
	}

	var app AppManifest
	if err := yaml.Unmarshal(data, &app); err != nil {
		log.Printf("Failed to parse YAML %s: %v", path, err)
		return
	}

	if app.Domain == "" || app.Port == 0 {
		log.Printf("Invalid manifest %s: missing domain or port", path)
		return
	}

	// 2. Generate Nginx Config
	safeName := strings.ReplaceAll(app.Domain, ":", "_")
	confName := fmt.Sprintf("%s.conf", safeName)
	confContent := w.generateNginxConfig(app)

	// 3. Save to /etc/nginx/conf.d/
	log.Printf("Generating config for %s -> %s", app.Domain, confName)
	if err := w.Manager.SaveConfig(confName, confContent); err != nil {
		log.Printf("Failed to save config: %v", err)
		return
	}

	// 4. Enable if directory configured
	if w.Manager.EnabledDir != "" {
		log.Printf("Enabling site %s", app.Domain)
		if err := w.Manager.EnableSite(confName); err != nil {
			log.Printf("Failed to enable site: %v", err)
		}
	}

	// 5. Test and Reload
	if err := w.Manager.TestConfig(); err != nil {
		log.Printf("Config invalid, not reloading: %v", err)
		// Optionally rollback? For now just logging error.
		return
	}

	if err := w.Manager.Reload(); err != nil {
		log.Printf("Reload failed: %v", err)
	} else {
		log.Printf("Successfully deployed %s", app.Domain)
	}
}

func (w *Watcher) generateNginxConfig(app AppManifest) string {
	protocol := app.Protocol
	if protocol == "" {
		protocol = "http"
	}
	hostname := app.Hostname
	if hostname == "" {
		hostname = "127.0.0.1"
	}

	listenPort := w.NginxListenPort
	serverName := app.Domain

	// If domain has port (e.g. localhost:3001), use that as listen port
	if host, portStr, err := net.SplitHostPort(app.Domain); err == nil {
		serverName = host
		if p, err := strconv.Atoi(portStr); err == nil {
			listenPort = p
		}
	}

	return fmt.Sprintf(`server {
    listen %d;
    server_name %s;

    location / {
        proxy_pass %s://%s:%d;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
`, listenPort, serverName, protocol, hostname, app.Port)
}
