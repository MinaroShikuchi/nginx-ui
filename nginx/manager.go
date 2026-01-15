package nginx

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tufanbarisyildirim/gonginx/config"
	"github.com/tufanbarisyildirim/gonginx/parser"
)

// Constants for Nginx paths - customizable for dev vs prod
const (
	NginxConfigPath = "/etc/nginx/nginx.conf"
	SitesConfigPath = "/etc/nginx/conf.d"
)

type Manager struct {
	ConfigDir      string // Now treated as primary directory (sites-available)
	EnabledDir     string // sites-enabled
	ArchivedDir    string // sites-archived
	NginxBinPath   string
	MainConfigPath string
}

func NewManager(configDir string, enabledDir string, archivedDir string, nginxBinPath string, mainConfigPath string) *Manager {
	if configDir == "" {
		configDir = SitesConfigPath
	}
	if enabledDir == "" {
		// Default to sites-enabled if available/enabled pattern is intended
		enabledDir = "/etc/nginx/sites-enabled"
	}
	if archivedDir == "" {
		archivedDir = "/etc/nginx/sites-archived"
	}
	if nginxBinPath == "" {
		nginxBinPath = "nginx"
	}
	if mainConfigPath == "" {
		mainConfigPath = NginxConfigPath
	}
	return &Manager{
		ConfigDir:      configDir,
		EnabledDir:     enabledDir,
		ArchivedDir:    archivedDir,
		NginxBinPath:   nginxBinPath,
		MainConfigPath: mainConfigPath,
	}
}

type SiteInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Url        string `json:"url"`
	Upstream   string `json:"upstream"`
	IsActive   bool   `json:"isActive"`
	HasSSL     bool   `json:"hasSsl"`
	IsEnabled  bool   `json:"isEnabled"`
	IsArchived bool   `json:"isArchived"`
}

// checkSiteStatus performs a quick HTTP GET to verify the site
func (m *Manager) checkSiteStatus(url string, domain string) bool {
	client := http.Client{
		Timeout: 2 * time.Second,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}
	// Important: Set Host header to ensure we hit the correct server block
	// validation against default_server serving 200 OK for everything.
	if domain != "" && domain != "_" {
		req.Host = domain
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// extractSiteDetails parses the config to find the first listen port, server_name and SSL status
// Returns simplified URL for checking (e.g. http://127.0.0.1:8080), domain and hasSSL
func (m *Manager) extractSiteDetails(path string) (string, string, bool) {
	p, err := parser.NewParser(path)
	if err != nil {
		return "", "", false
	}
	conf, err := p.Parse()
	if err != nil {
		return "", "", false
	}

	for _, d := range conf.Block.Directives {
		if d.GetName() == "server" {
			return m.parseServerBlock(d.GetBlock())
		}
		if d.GetName() == "http" {
			if d.GetBlock() != nil {
				for _, hDirective := range d.GetBlock().GetDirectives() {
					if hDirective.GetName() == "server" {
						return m.parseServerBlock(hDirective.GetBlock())
					}
				}
			}
		}
	}
	return "", "", false
}

func (m *Manager) parseServerBlock(block config.IBlock) (string, string, bool) {
	if block == nil {
		return "http://127.0.0.1:80", "", false
	}
	port := "80"
	domain := ""
	hasSSL := false

	for _, d := range block.GetDirectives() {
		if d.GetName() == "listen" {
			params := d.GetParameters()
			if len(params) > 0 {
				port = params[0].Value
				// Check for 'ssl' parameter in listen directive
				for _, p := range params {
					if p.Value == "ssl" {
						hasSSL = true
					}
				}
			}
		}
		if d.GetName() == "server_name" {
			if len(d.GetParameters()) > 0 {
				domain = d.GetParameters()[0].Value
			}
		}
		if d.GetName() == "ssl_certificate" {
			hasSSL = true
		}
	}

	return fmt.Sprintf("http://127.0.0.1:%s", port), domain, hasSSL
}

// GetProxyTarget parses the config to find the proxy_pass directive
// Returns protocol, hostname, port, and error
func (m *Manager) GetProxyTarget(filename string) (string, string, int, error) {
	path := m.resolvePath(filename)
	p, err := parser.NewParser(path)
	if err != nil {
		return "", "", 0, err
	}
	conf, err := p.Parse()
	if err != nil {
		return "", "", 0, err
	}

	var findProxyPass func(block config.IBlock) (string, error)
	findProxyPass = func(block config.IBlock) (string, error) {
		for _, d := range block.GetDirectives() {
			if d.GetName() == "location" {
				// Search inside location block
				if d.GetBlock() != nil {
					for _, ld := range d.GetBlock().GetDirectives() {
						if ld.GetName() == "proxy_pass" {
							if len(ld.GetParameters()) > 0 {
								return ld.GetParameters()[0].Value, nil
							}
						}
					}
				}
			}
			// Recursive search if nested (though strictly proxy_pass is usually in location)
			if d.GetBlock() != nil {
				if res, err := findProxyPass(d.GetBlock()); err == nil {
					return res, nil
				}
			}
		}
		return "", fmt.Errorf("proxy_pass not found")
	}

	// Search in http -> server context
	for _, d := range conf.Block.Directives {
		if d.GetName() == "server" {
			if target, err := findProxyPass(d.GetBlock()); err == nil {
				return parseProxyUrl(target)
			}
		}
		if d.GetName() == "http" {
			if d.GetBlock() != nil {
				for _, hDirective := range d.GetBlock().GetDirectives() {
					if hDirective.GetName() == "server" {
						if target, err := findProxyPass(hDirective.GetBlock()); err == nil {
							return parseProxyUrl(target)
						}
					}
				}
			}
		}
	}
	return "", "", 0, fmt.Errorf("no proxy target found")
}

func parseProxyUrl(rawUrl string) (string, string, int, error) {
	// Expected formats: http://localhost:3000, http://127.0.0.1:8080
	// Strip trailing slash/path if any (simple implementation)
	// We only want the root

	// Remove variable references if simple ones exist like $host, but for reverse discovery we assume simple static for now
	// or best effort.

	// Check for protocol
	protocol := "http"
	if strings.HasPrefix(rawUrl, "https://") {
		protocol = "https"
		rawUrl = strings.TrimPrefix(rawUrl, "https://")
	} else {
		rawUrl = strings.TrimPrefix(rawUrl, "http://")
	}

	// Split host and port
	host, portStr, err := net.SplitHostPort(rawUrl)
	if err != nil {
		// Maybe no port specified, default to 80 (or just return error if we expect port)
		// Try to handle "localhost" -> localhost, 0? Or assume 80?
		// For our AppManifest, we need a port.
		// If strings.Contains(rawUrl, ":") failed, maybe it's just host.
		return protocol, rawUrl, 80, nil
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return protocol, host, 80, nil
	}

	return protocol, host, port, nil
}

// GetSites returns a list of active configurations with health checks
func (m *Manager) GetSites() ([]SiteInfo, error) {
	files, err := os.ReadDir(m.ConfigDir)
	var rawSites []string

	// Track archived status map
	archivedMap := make(map[string]bool)

	if err == nil {
		for _, f := range files {
			if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
				// Accept all non-hidden, non-directory files
				// Many systems use extensionless files in sites-available
				rawSites = append(rawSites, f.Name())
			}
		}
	}

	// Also scan archived sites
	if m.ArchivedDir != "" {
		archivedFiles, err := os.ReadDir(m.ArchivedDir)
		if err == nil {
			for _, f := range archivedFiles {
				if !f.IsDir() && strings.HasSuffix(f.Name(), ".conf") {
					rawSites = append(rawSites, f.Name())
					archivedMap[f.Name()] = true
				}
			}
		}
	}

	// Prepend main config (only if not archived, which is weird, but main shouldn't be moved)
	rawSites = append([]string{"nginx.conf"}, rawSites...)

	// Result channel for concurrency
	type result struct {
		index int
		info  SiteInfo
	}
	results := make(chan result, len(rawSites))
	var wg sync.WaitGroup

	for i, filename := range rawSites {
		wg.Add(1)
		go func(idx int, fname string) {
			defer wg.Done()

			isArchived := archivedMap[fname]

			// Resolve path based on whether it is archived or available
			var fullPath string
			if isArchived {
				fullPath = filepath.Join(m.ArchivedDir, fname)
			} else {
				fullPath = m.resolvePath(fname)
			}

			// url here is the "internal" check URL (http://127.0.0.1:port)
			checkUrl, domain, hasSSL := m.extractSiteDetails(fullPath)

			// Construct the public display URL
			displayUrl := checkUrl
			if domain != "" && domain != "_" {
				protocol := "http"
				if hasSSL {
					protocol = "https"
				}
				displayUrl = fmt.Sprintf("%s://%s", protocol, domain)
			}

			// Try to get Upstream
			upstreamProto, upstreamHost, upstreamPort, err := m.GetProxyTarget(fname)
			upstream := ""
			if err == nil {
				upstream = fmt.Sprintf("%s://%s:%d", upstreamProto, upstreamHost, upstreamPort)
			}

			active := false
			if checkUrl != "" {
				active = m.checkSiteStatus(checkUrl, domain)
			} else {
				// Fallback/Unknown, maybe just a partial config
				displayUrl = "N/A"
			}

			// Check if enabled (symlink exists in EnabledDir)
			enabled := true // Default true for legacy or main config
			if m.EnabledDir != "" && fname != "nginx.conf" && !isArchived {
				enabledPath := filepath.Join(m.EnabledDir, fname)
				if _, err := os.Lstat(enabledPath); err != nil {
					enabled = false
				}
			} else if isArchived {
				enabled = false
			}

			results <- result{
				index: idx,
				info: SiteInfo{
					Name:       fname,
					Path:       fullPath,
					Url:        displayUrl,
					Upstream:   upstream,
					IsActive:   active,
					HasSSL:     hasSSL,
					IsEnabled:  enabled,
					IsArchived: isArchived,
				},
			}
		}(i, filename)
	}

	wg.Wait()
	close(results)

	var sites []SiteInfo
	for res := range results {
		sites = append(sites, res.info)
	}

	sort.Slice(sites, func(i, j int) bool {
		return sites[i].Name < sites[j].Name
	})

	return sites, nil
}

// ArchiveSite moves a site from available to archived
func (m *Manager) ArchiveSite(name string) error {
	if name == "nginx.conf" {
		return fmt.Errorf("cannot archive main nginx.conf")
	}
	// Ensure directories
	if err := os.MkdirAll(m.ArchivedDir, 0755); err != nil {
		return err
	}

	src := filepath.Join(m.ConfigDir, name)
	dst := filepath.Join(m.ArchivedDir, name)

	// Disable first
	_ = m.DisableSite(name)

	return os.Rename(src, dst)
}

// RestoreSite moves a site from archived to available
func (m *Manager) RestoreSite(name string) error {
	src := filepath.Join(m.ArchivedDir, name)
	dst := filepath.Join(m.ConfigDir, name)

	// Check if already exists in available
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("site %s already exists in available sites", name)
	}

	return os.Rename(src, dst)
}

// EnableSite creates a symlink from available to enabled
func (m *Manager) EnableSite(name string) error {
	if name == "nginx.conf" {
		return fmt.Errorf("cannot toggle main nginx.conf")
	}
	if m.EnabledDir == "" {
		return fmt.Errorf("enabled directory not configured")
	}

	// Ensure enabled directory exists
	if err := os.MkdirAll(m.EnabledDir, 0755); err != nil {
		return fmt.Errorf("failed to create enabled directory: %v", err)
	}

	availablePath, _ := filepath.Abs(filepath.Join(m.ConfigDir, name))
	enabledPath, _ := filepath.Abs(filepath.Join(m.EnabledDir, name))

	// Check if link already exists
	if _, err := os.Lstat(enabledPath); err == nil {
		return nil // Already enabled
	}

	return os.Symlink(availablePath, enabledPath)
}

// DisableSite removes the symlink from enabled
func (m *Manager) DisableSite(name string) error {
	if name == "nginx.conf" {
		return fmt.Errorf("cannot toggle main nginx.conf")
	}
	if m.EnabledDir == "" {
		return fmt.Errorf("enabled directory not configured")
	}

	enabledPath := filepath.Join(m.EnabledDir, name)
	return os.Remove(enabledPath)
}

// resolvePath helper to handle 'nginx.conf' as a special virtual file
func (m *Manager) resolvePath(filename string) string {
	if filename == "nginx.conf" {
		return m.MainConfigPath
	}
	return filepath.Join(m.ConfigDir, filename)
}

// ParseConfig reads and parses a specific config file
func (m *Manager) ParseConfig(filename string) (*config.Config, error) {
	path := m.resolvePath(filename)
	p, err := parser.NewParser(path)
	if err != nil {
		return nil, err
	}
	return p.Parse()
}

// GetConfig reads the raw content of a config file
func (m *Manager) GetConfig(filename string) (string, error) {
	path := m.resolvePath(filename)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// SaveConfig writes raw config to a file
func (m *Manager) SaveConfig(filename, content string) error {
	path := m.resolvePath(filename)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// TestConfig runs nginx -t
func (m *Manager) TestConfig() error {
	cmd := exec.Command(m.NginxBinPath, "-t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nginx configuration invalid: %s: %v", string(out), err)
	}
	return nil
}

// Reload runs nginx -s reload (portable)
func (m *Manager) Reload() error {
	cmd := exec.Command(m.NginxBinPath, "-s", "reload")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to reload nginx: %s: %v", string(out), err)
	}
	return nil
}

// Certbot runs certbot for a given domain
// Assumes certbot-nginx plugin is installed
func (m *Manager) RunCertbot(domain string) error {
	// Non-interactive, agree to tos, etc.
	cmd := exec.Command("certbot", "--nginx", "-d", domain, "--non-interactive", "--agree-tos", "--register-unsafely-without-email")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certbot failed: %s: %v", string(out), err)
	}
	return nil
}
