# Binary name
BINARY_NAME=autonomous_ci_platform

# Build directories
BUILD_DIR=out

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean

.PHONY: all build clean linux windows run-windows

# Default task builds for all platforms
all: clean linux windows

# Linux build
linux:
	@echo Building for Linux...
	set GOOS=linux&& set GOARCH=amd64&& $(GOBUILD) -o $(BUILD_DIR)\$(BINARY_NAME)-linux-amd64 cmd\main.go

# Windows build
windows:
	@echo Building for Windows...
	set GOOS=windows&& set GOARCH=amd64&& $(GOBUILD) -o $(BUILD_DIR)\$(BINARY_NAME).exe cmd\main.go

# Clean build artifacts (Windows Command Prompt compatible)
clean:
	@echo Cleaning build directory...
	$(GOCLEAN)
	if exist $(BUILD_DIR) rmdir /s /q $(BUILD_DIR)

run-windows: clean windows
	@echo Starting ngrok and app...
	powershell -Command "Start-Process ngrok -ArgumentList 'http 8080'"
	$(BUILD_DIR)\$(BINARY_NAME).exe