package server

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/MinaroShikuchi/nginx-ui/nginx"
	"github.com/gin-gonic/gin"
)

type Server struct {
	Manager *nginx.Manager
	Router  *gin.Engine
	FS      embed.FS
	AppsDir string
}

func NewServer(mgr *nginx.Manager, appsDir string, frontendFS embed.FS) *Server {
	r := gin.Default()
	s := &Server{
		Manager: mgr,
		Router:  r,
		FS:      frontendFS,
		AppsDir: appsDir,
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	api := s.Router.Group("/api")
	{
		api.GET("/sites", s.handleGetSites)
		api.GET("/sites/:name", s.handleGetSite)
		api.POST("/sites", s.handleSaveSite)
		api.POST("/sites/:name/toggle", s.handleToggleSite)
		api.POST("/sites/:name/archive", s.handleArchiveSite)
		api.POST("/sites/:name/restore", s.handleRestoreSite)
		api.POST("/apps", s.handleCreateApp)
		api.POST("/ssl", s.handleSSL)
		api.GET("/health", s.handleHealth)
	}

	// Serve Frontend
	// 1. Static Assets: /assets -> frontend/dist/assets
	assetsFS, err := fs.Sub(s.FS, "frontend/dist/assets")
	if err == nil {
		s.Router.StaticFS("/assets", http.FS(assetsFS))
	} else {
		// Dev fallback
		s.Router.Static("/assets", "./frontend/dist/assets")
	}

	// 2. SPA Fallback: Everything else -> index.html
	s.Router.NoRoute(func(c *gin.Context) {
		// Try to read index.html from embed
		index, err := s.FS.ReadFile("frontend/dist/index.html")
		if err != nil {
			// Try local file (dev mode)
			index, err = os.ReadFile("./frontend/dist/index.html")
		}

		if err != nil {
			c.String(http.StatusNotFound, "index.html not found")
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
}

func (s *Server) handleGetSites(c *gin.Context) {
	sites, err := s.Manager.GetSites()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sites": sites})
}

func (s *Server) handleGetSite(c *gin.Context) {
	name := c.Param("name")
	content, err := s.Manager.GetConfig(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Config not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": content})
}

type SaveSiteRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func (s *Server) handleSaveSite(c *gin.Context) {
	var req SaveSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.Manager.SaveConfig(req.Name, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Test config
	if err := s.Manager.TestConfig(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Config: " + err.Error()})
		return
	}

	// Reload
	if err := s.Manager.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reload Failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type SSLRequest struct {
	Domain string `json:"domain"`
}

func (s *Server) handleSSL(c *gin.Context) {
	var req SSLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.Manager.RunCertbot(req.Domain); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ssl installed"})
}

type ToggleSiteRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleToggleSite(c *gin.Context) {
	name := c.Param("name")
	var req ToggleSiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var err error
	if req.Enabled {
		err = s.Manager.EnableSite(name)
	} else {
		err = s.Manager.DisableSite(name)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Test and Reload
	if err := s.Manager.TestConfig(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Config Invalid: " + err.Error()})
		return
	}
	if err := s.Manager.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Reload Failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleArchiveSite(c *gin.Context) {
	name := c.Param("name")
	if err := s.Manager.ArchiveSite(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Reload after archiving (since it disables too)
	if err := s.Manager.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Archive successful but reload failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "archived"})
}

func (s *Server) handleRestoreSite(c *gin.Context) {
	name := c.Param("name")
	if err := s.Manager.RestoreSite(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "restored"})
}

type CreateAppRequest struct {
	Domain   string `json:"domain"`
	Protocol string `json:"protocol"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
}

func (s *Server) handleCreateApp(c *gin.Context) {
	var req CreateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Domain == "" || req.Port == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain and port are required"})
		return
	}

	// Create YAML content
	content := fmt.Sprintf("domain: %s\nprotocol: %s\nhostname: %s\nport: %d\n",
		req.Domain, req.Protocol, req.Hostname, req.Port)
	safeName := strings.ReplaceAll(req.Domain, ":", "_")
	filename := fmt.Sprintf("%s.yaml", safeName)
	path := filepath.Join(s.AppsDir, filename)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write manifest: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "manifest created", "path": path})
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
