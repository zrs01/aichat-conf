# aichatconf

A simple configuration tool for [aichat](https://github.com/sigoden/aichat) that automatically syncs Ollama models to your aichat configuration.

## Overview

`aichatconf` reads your existing aichat configuration file, discovers available Ollama models, and automatically adds missing models with their proper parameters (temperature, top_p, max_input_tokens) to the configuration.

## Features

- Automatically discovers Ollama models
- Supports Ollama running locally
- Supports Ollama API base URL via environment variable
- Extracts model parameters from Ollama model info
- Removes obsolete models from configuration
- Adds missing models to aichat configuration
- Supports model exclusion via command line
- Preserves existing configuration structure and comments
- Supports writing output to file
- Supports sorting models by name

## Installation

### From Source

```bash
git clone https://github.com/zrs01/aichat-conf
cd aichat-conf
make
```

### Cross-platform Builds

```bash
make build  # Builds for Windows, Linux, and macOS
```

## Usage

```bash
aichatconf -c /path/to/aichat/config.yaml
```

### Options

- `-c, --config`: Path to aichat configuration file (required)
- `-n, --client`: Client name, default is "ollama"
- `-e, --exclude`: Comma-separated list of models to exclude
- `-o, --output`: Output file, default is stdout
- `-q, --quite`: Suppress all information output
- `-d, --debug`: Enable debug mode
- `-h, --help`: Show help

### Examples

```bash
# Basic usage
aichatconf -c ~/.config/aichat/config.yaml

# Exclude specific models
aichatconf -c ~/.config/aichat/config.yaml -e "llama3,mistral"

# Write output to file
aichatconf -c ~/.config/aichat/config.yaml -o /path/to/output.yaml
```

## Requirements

- Go 1.24.5+
- Existing aichat configuration with an "ollama" client

## How it Works

1. Reads your aichat configuration file
2. Finds the "ollama" client configuration
   - Supports Ollama API base URL via environment variable
3. Queries Ollama API for available models
4. For each obsolete model, remove it from the configuration
5. For each missing model:
   - Extracts context length from model info
   - Parses temperature and top_p from model parameters
   - Adds model to configuration
6. Outputs updated configuration to stdout or file

## Development

```bash
# Install dependencies
go mod download

# Run with file watching
make  # Uses modd for auto-rebuild

# Run tests (when available)
make test
```

## License

[MIT License](LICENSE)
