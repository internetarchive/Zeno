# Zeno Web Crawler

Zeno is a state-of-the-art web crawler written in Go designed to operate wide crawls or to simply archive one web page. It records traffic into WARC files and emphasizes portability, performance, and simplicity.

Always reference these instructions first and fallback to search or bash commands only when you encounter unexpected information that does not match the info here.

## Working Effectively

### Prerequisites and Dependencies
- **Go 1.25+** is required (specified in go.mod)
- **CGO is REQUIRED** - this project depends on C++ libraries 
- **C++ compiler** (g++ or equivalent) is required for building
- **Linux/amd64** is the primary supported platform
- Dependencies: `libstdc++` and `libgcc` runtime libraries

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

### Testing Scenarios
- **CLI functionality**: Test `--help` commands and basic argument parsing
- **Build verification**: Ensure CGO linking works and binary runs
- **Cross-platform builds**: Project supports Linux, Windows via zig toolchain
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
- **CGO requirement**: Build will fail without C++ compiler and libraries

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

## Common Workflows

### Adding New Features
1. Build and test baseline: `go build -o Zeno . && ./Zeno --help`
2. Make focused changes to relevant packages
3. Run tests: `go test ./...` (exclude e2e if needed)
4. Verify build: `go build -o Zeno .`
5. Test CLI functionality: `./Zeno --help`
6. Format code: `gofmt -w .` 
7. Run vet: `go vet ./...`

### Debugging Build Issues
- **CGO errors**: Ensure C++ compiler (g++) is installed
- **Missing symbols**: Check that libstdc++ and libgcc are available
- **Network timeouts**: Expected in sandboxed environments for external tests
- **Test failures**: Most network-dependent test failures are expected in restricted environments

### Cross-Platform Development  
- **Linux builds**: Standard `go build` (primary platform)
- **Windows builds**: Use zig toolchain as configured in CI
- **ARM64 builds**: Use zig toolchain as configured in CI
- See `.github/workflows/build.yml` for exact cross-compilation commands

## CI/CD Integration
- **Main CI**: `.github/workflows/go.yml` runs full test suite
- **Release builds**: `.github/workflows/build.yml` creates cross-platform binaries
- **Coverage reporting**: Integrated with Codecov for test coverage tracking
- **Goroutine leak detection**: Automated checks for resource leaks