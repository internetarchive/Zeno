# Zeno Web Crawler

Zeno is a state-of-the-art web crawler written in Go designed to operate wide crawls or to simply archive one web page. It records traffic into WARC files and emphasizes portability, performance, and simplicity.

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

### Prerequisites and Dependencies
- **Go 1.25+** is required (specified in go.mod)
- **Linux/amd64** is the primary supported platform, but also builds on:
  - **Linux/arm64**
  - **Windows/amd64**

### Build and Test Commands
- **CRITICAL**: NEVER CANCEL long-running commands. Set timeouts appropriately.

#### Initial Setup
```bash
go version  # Verify Go 1.25+ is installed
go mod download  # Takes ~25 seconds, NEVER CANCEL
```

#### Build the Project
```bash
go build -v ./...  # Takes ~50 seconds, NEVER CANCEL - set timeout to 90+ minutes for safety
go build -o Zeno .  # Build main executable, takes ~3 seconds
```

#### Test the Project
```bash
# Setup test environment (required for e2e tests)
mkdir -p /tmp/unit_coverage /tmp/e2e_coverage
echo 0 | sudo tee /proc/sys/kernel/apparmor_restrict_unprivileged_userns

# Unit tests - Takes ~85 seconds, NEVER CANCEL - set timeout to 30+ minutes
go test -v -p=12 -race -cover -covermode=atomic -coverpkg=./... $(go list ./... | grep -v /e2e/test/) -args -test.gocoverdir=/tmp/unit_coverage

# E2E tests - Takes ~47 seconds, NEVER CANCEL - set timeout to 40+ minutes  
go test -v -p=12 -race -cover -covermode=atomic -coverpkg=./... $(go list ./e2e/test/...) -args -test.gocoverdir=/tmp/e2e_coverage
```

#### Code Quality
```bash
go vet ./...        # Fast linting check
gofmt -l .          # Check formatting (some files may need formatting)
```

### Run the Application

#### CLI Usage
```bash
./Zeno --help               # Show main help
./Zeno get --help           # Show crawling options  
./Zeno get url --help       # Show URL archiving help
./Zeno get url https://example.com  # Archive a single URL
```

#### Available Commands
- `Zeno get url [URL...]` - Archive given URLs to WARC files
- `Zeno get hq` - Start crawling with crawl HQ connector
- Use `--tui` flag for terminal user interface

### Docker Support
- **Docker build currently fails** in sandbox environments due to network restrictions
- Dockerfile exists but requires internet access for Alpine package installation
- Use local Go build instead of Docker for development

## Validation and Testing

### Manual Validation Steps
After making changes, ALWAYS:
1. **Build successfully**: `go build -o Zeno .`
2. **Run help command**: `./Zeno --help` to verify CLI works
3. **Check binary dependencies**: `ldd ./Zeno` to verify C++ linkage
4. **Run formatting check**: `gofmt -l .` and fix any issues
5. **Run vet**: `go vet ./...` to catch common issues
6. **Try to crawl example website**: `./Zeno get url https://example.com --workers 1` to test basic archiving functionality
7. **(if applicable) attempt to crawl using HQ as well**: `./Zeno get hq --help` to verify HQ connector works

### Testing Scenarios
- **CLI functionality**: Test `--help` commands and basic argument parsing
- **Build verification**: Ensure `go build` works and binary runs
- **Cross-platform builds**: Project supports Linux and Windows
- **Network tests may fail** in restricted environments (this is expected)

## Critical Build Information

### Timing Expectations (with 50% safety buffer)
- **Initial go mod download**: ~25 seconds (timeout: 600 seconds)
- **Full build (go build -v ./...)**: ~50 seconds (timeout: 1800 seconds)  
- **Main binary build**: ~3 seconds (timeout: 300 seconds)
- **Unit tests**: ~85 seconds (timeout: 1800 seconds)
- **E2E tests**: ~47 seconds (timeout: 2400 seconds)

### Known Issues and Workarounds
- **Network-dependent tests fail** in sandbox/restricted environments (expected)
- **Docker build fails** without internet access to Alpine repositories  
- **Some files need formatting**: Run `gofmt -w .` if needed

## Project Structure

### Key Directories
- `cmd/` - CLI command definitions and parsing
- `internal/pkg/` - Internal packages (archiver, postprocessor, etc.)
- `pkg/models/` - Shared data models and utilities
- `e2e/` - End-to-end tests and test utilities
- `.github/workflows/` - CI/CD pipeline definitions

### Important Files
- `main.go` - Application entry point
- `go.mod` - Go module definition with Go 1.25 requirement
- `Dockerfile` - Container build definition (requires network access)
- `sqlc.yaml` - SQL code generation configuration
- `.github/workflows/go.yml` - Main CI pipeline with comprehensive testing

### Main Components
- **Archiver**: Core web crawling and WARC recording engine
- **Postprocessor**: Content processing and extraction
- **Preprocessor**: URL preprocessing and deduplication  
- **Source**: URL queue management (local queue, HQ connector)
- **Reactor**: Event processing pipeline
- **UI**: Terminal user interface components

## Development Best Practices

### Binary and Artifact Management
- **NEVER commit Zeno binaries** to the repository - they are already excluded in `.gitignore`
- **Use `.gitignore`** to exclude build artifacts, dependencies, and temporary files
- **Clean builds**: Remove old binaries before building new ones to avoid confusion

### HTTP Client Usage
- **ALWAYS use the WARC HTTP client** from the gowarc package for all archival requests
- **Use `warc.CustomHTTPClient`** (available as `Client` and `ClientWithProxy` in the archiver)
- **Never use standard `http.Client`** for requests that need to be archived to WARC files
- **Read HTTP response buffers in full** and **close response bodies** to prevent resource leaks
- **Example pattern**:
  ```go
  resp, err := warcClient.Do(req)
  if err != nil {
      return err
  }
  defer resp.Body.Close()
  
  // Read the full response body
  body, err := io.ReadAll(resp.Body)
  if err != nil {
      return err
  }
  // Process the body...
  ```

### Resource Management
- **Always close HTTP response bodies** using `defer resp.Body.Close()`
- **Read buffers completely** to ensure proper connection pooling and resource cleanup
- **Handle context cancellation** properly in long-running operations
- **Monitor for goroutine leaks** during development and testing

## Common Workflows

### Adding New Features
1. Build and test baseline: `go build -o Zeno . && ./Zeno --help`
2. Make focused changes to relevant packages
3. **Ensure tests are added for new features**: Write unit/integration tests for any new functionality
4. Run tests: `go test ./...` (exclude e2e if needed)
5. Verify build: `go build -o Zeno .`
6. Test CLI functionality: `./Zeno --help`
7. Format code: `gofmt -w .` 
8. Run vet: `go vet ./...`

### Debugging Build Issues
- **Network timeouts**: Expected in sandboxed environments for external tests
- **Test failures**: Most network-dependent test failures are expected in restricted environments

### Cross-Platform Development  
- **Linux builds**: Standard `go build` (primary platform)
- **Windows builds**: Use `GOOS=windows GOARCH=amd64 go build -v ./...`
- **ARM64 builds**: Use `GOOS=linux GOARCH=arm64 go build -v ./...`
- See `.github/workflows/build.yml` for exact cross-compilation commands

## CI/CD Integration
- **Main CI**: `.github/workflows/go.yml` runs full test suite
- **Release builds**: `.github/workflows/build.yml` creates cross-platform binaries
- **Coverage reporting**: Integrated with Codecov for test coverage tracking
- **Goroutine leak detection**: Automated checks for resource leaks