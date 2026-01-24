# Nginx UI

Nginx UI is a powerful management tool for Nginx, capable of automatically discovering applications, managing configuration files, and providing a modern web interface for monitoring and control.

## Features

- **Web Interface**: A built-in Vue.js-based dashboard to manage your Nginx server visually.
  - **Live Status**: View active sites, their public URLs, and upstream targets.
  - **Quick Actions**: Enable, disable, or archive sites with a toggle.
- **Auto-Discovery (Apps Folder)**:
  - The `apps` folder is a high-level abstraction. You drop simple YAML files here (e.g., defining just domain and port), and Nginx UI **automatically generates** the complex Nginx configuration files in `sites-available`.
- **Reverse Discovery (Sync)**:
  - Existing Nginx configurations (even those manually created or without extensions) are automatically parsed and synced back to the `apps` folder as YAML manifests, ensuring a two-way synchronization.
- **Config Management**: Manage standard Nginx configurations found in `sites-available`.
- **Interactive CLI**: Control the server directly from the terminal with keyboard shortcuts.
- **Cross-Platform**: Smart defaults for Linux and macOS (Homebrew structure).
- **Single Binary**: The frontend is embedded into the Go binary, making deployment as simple as copying a single file.

## Installation

### From Releases
Download the latest binary for your operating system from the [Releases](https://github.com/MinaroShikuchi/nginx-ui/releases) page.

#### Linux Installation
You can install the latest release with the following commands:

```bash
# 1. Download the latest release (replace v0.0.1 with the actual version)
curl -L -o nginx-ui.tar.gz https://github.com/MinaroShikuchi/nginx-ui/releases/download/v0.0.1/nginx-ui_Linux_x86_64.tar.gz

# 2. Extract the archive
tar -xzf nginx-ui.tar.gz

# 3. Move the binary to your path
sudo mv nginx-ui /usr/local/bin/

# 4. Verify installation
nginx-ui --help
```

### Build from Source
Requirements:
- Go 1.25+
- Node.js 20+ (for frontend)

```bash
# 1. Build the frontend
cd frontend
npm install
npm run build
cd ..

# 2. Build the backend (embeds the frontend)
go build -v -o nginx-ui
```

## Usage

Start the server with default settings:
```bash
sudo ./nginx-ui
```
*Note: Sudo is usually required to modify Nginx configuration files in `/etc/nginx`.*

### Command Line Flags

| Flag | Description | Default (Linux) | Default (macOS) |
|------|-------------|-----------------|-----------------|
| `--port` | Port for the Nginx UI Dashboard | `9000` | `9000` |
| `--apps` | Directory to watch for app manifests | `./apps` | `./apps` |
| `--available-dir` | Nginx `sites-available` directory | `/etc/nginx/sites-available` | `/usr/local/etc/nginx/sites-available` |
| `--enabled-dir` | Nginx `sites-enabled` directory | `/etc/nginx/sites-enabled` | `/usr/local/etc/nginx/sites-enabled` |
| `--archived-dir` | Directory for archived configs | `/etc/nginx/sites-archived` | `/usr/local/etc/nginx/sites-archived` |
| `--nginx-bin` | Path to Nginx binary | `nginx` | `/usr/local/opt/nginx/bin/nginx` |
| `--nginx-port` | Port for generated Nginx configs | `80` | `8080` |
| `--main-config` | Path to main `nginx.conf` | `/etc/nginx/nginx.conf` | `/usr/local/etc/nginx/nginx.conf` |
| `--server` | Disable interactive shortcuts (for services) | `false` | `false` |

### Interactive Shortcuts

When the application is running in the terminal, you can use the following keys:
- **`r`**: Reload Nginx configuration.
- **`R`**: Full System Trigger (Test config & Reload).
- **`q`**: Quit the application.

## Development

To run the project in development mode:

1. **Frontend**:
   ```bash
   cd frontend
   npm run dev
   ```
   This will start the Vite dev server.

2. **Backend**:
   ```bash
   go run main.go
   ```

## License

[MIT](LICENSE)
