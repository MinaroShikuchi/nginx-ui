package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"os"

	"runtime"

	"github.com/MinaroShikuchi/nginx-ui/discovery"
	"github.com/MinaroShikuchi/nginx-ui/nginx"
	"github.com/MinaroShikuchi/nginx-ui/server"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

func main() {
	// Platform-specific defaults
	defConfigDir := "/etc/nginx/conf.d"
	defEnabledDir := "/etc/nginx/sites-enabled"
	defArchivedDir := "/etc/nginx/sites-archived"
	defNginxBin := "nginx"
	defMainConfig := "/etc/nginx/nginx.conf"
	defNginxPort := 80

	if runtime.GOOS == "darwin" {
		prefix := "/usr/local" // Default Intel Mac Homebrew prefix
		if runtime.GOARCH == "arm64" {
			prefix = "/opt/homebrew" // Apple Silicon Homebrew prefix
		}

		// Verify if the prefix actually exists, otherwise fallback to standard
		if _, err := os.Stat(prefix); err == nil {
			defConfigDir = prefix + "/etc/nginx/sites-available" // Changed from /servers to be consistent
			defEnabledDir = prefix + "/etc/nginx/sites-enabled"
			defArchivedDir = prefix + "/etc/nginx/sites-archived"
			defNginxBin = prefix + "/opt/nginx/bin/nginx"
			defMainConfig = prefix + "/etc/nginx/nginx.conf"
			defNginxPort = 8080 // Homebrew Nginx usually runs on 8080 by default to avoid sudo
		}
	} else {
		// Linux defaults often use sites-available/enabled too
		defConfigDir = "/etc/nginx/sites-available"
	}

	appsDir := flag.String("apps", "./apps", "Directory to watch for app manifests")
	configDir := flag.String("available-dir", defConfigDir, "Directory for Nginx configs (sites-available)")
	enabledDir := flag.String("enabled-dir", defEnabledDir, "Directory for enabled Nginx configs (sites-enabled)")
	archivedDir := flag.String("archived-dir", defArchivedDir, "Directory for archived Nginx configs")
	nginxBin := flag.String("nginx-bin", defNginxBin, "Path to Nginx binary")
	nginxPort := flag.Int("nginx-port", defNginxPort, "Port for generated Nginx configs to listen on")
	paramsPort := flag.String("port", "9000", "Port for Nginx Manager Dashboard")
	mainConfig := flag.String("main-config", defMainConfig, "Path to main nginx.conf")
	flag.Parse()

	// 1. Initialize Nginx Manager
	log.Printf("Scanning Directory for Nginx configs: %s", *configDir)
	log.Printf("Directory for enabled Nginx configs: %s", *enabledDir)
	mgr := nginx.NewManager(*configDir, *enabledDir, *archivedDir, *nginxBin, *mainConfig)

	if sites, err := mgr.GetSites(); err == nil {
		log.Printf("Found %d available configurations:", len(sites))
		for _, site := range sites {
			status := "Disabled"
			if site.IsEnabled {
				status = "Enabled"
			}
			log.Printf(" - %s [%s]", site.Name, status)
		}
	} else {
		log.Printf("Error scanning sites: %v", err)
	}

	// 2. Start Autodiscovery Watcher
	watcher := discovery.NewWatcher(mgr, *appsDir, *nginxPort)
	go watcher.Start()

	// 3. Start API Server
	srv := server.NewServer(mgr, *appsDir, frontendFS)

	log.Printf("Starting Nginx Manager on :%s", *paramsPort)
	log.Println("Interactive Shortcuts: [r] Reload Nginx, [R] Full System Trigger, [q] Quit")

	// Keyboard Shortcuts Goroutine
	go func() {
		var input string
		for {
			_, err := fmt.Scanln(&input)
			if err != nil {
				continue
			}
			switch input {
			case "r":
				log.Println("Shortcut [r]: Reloading Nginx...")
				if err := mgr.Reload(); err != nil {
					log.Printf("Reload failed: %v", err)
				} else {
					log.Println("Reload successful")
				}
			case "R":
				log.Println("Shortcut [R]: Global System Trigger...")
				// Force test and reload
				if err := mgr.TestConfig(); err != nil {
					log.Printf("Test failed: %v", err)
				} else if err := mgr.Reload(); err != nil {
					log.Printf("Reload failed: %v", err)
				} else {
					log.Println("System triggered and reloaded successfully")
				}
			case "q":
				log.Println("Quitting...")
				os.Exit(0)
			}
		}
	}()

	if err := srv.Router.Run(":" + *paramsPort); err != nil {
		log.Fatal(err)
	}
}
