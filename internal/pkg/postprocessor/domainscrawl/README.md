# domainscrawl Package

## Overview

The `domainscrawl` package is a postprocessing component that enables Zeno to continuously crawl specific domains by matching URLs against predefined patterns. It allows Zeno to maintain an ongoing crawl of target domains for comprehensive coverage.

## Features

- **Multiple Pattern Types**: Supports naive domains, full URLs, and regex patterns
- **Efficient Matching**: Uses an Adaptive Radix Tree (ART) for fast domain lookups
- **Thread-Safe**: Implements proper locking for concurrent access
- **Flexible Input**: Accepts both direct string inputs and file-based inputs

## Usage

### Basic Example

```go
import (
    "github.com/internetarchive/Zeno/internal/pkg/postprocessor/domainscrawl"
)

// Reset the matcher to initial state
domainscrawl.Reset()

// Add patterns from strings and files
err := domainscrawl.AddElements(
    []string{"example.com", "https://test.org/path", `.*\\.gov`},
    []string{"patterns.txt"},
)
if err != nil {
    log.Fatal(err)
}

// Check if a URL matches any pattern
if domainscrawl.Match("https://sub.example.com/page") {
    fmt.Println("URL matches!")
}

// Check if matcher is enabled
enabled := domainscrawl.Enabled()
```

### Pattern Types

The package automatically detects and handles different pattern types:

1. **Naive Domains**: Simple domain names like `example.com`
2. **Full URLs**: Complete URLs like `https://test.org/path?query=value`
3. **Regex Patterns**: Regular expressions like `.*\\.gov`

### Matching Logic

The `Match` function checks URLs against stored patterns in this order:

1. Exact domain matches (O(1) lookup)
2. Subdomain matches (prefix search using an ART, O(n) where n is domain length)
3. Full URL matches (exact or greedy subdomain)
4. Regex pattern matches

## Implementation Details

### Data Structures

- **Adaptive Radix Tree (ART)**: Used for efficient domain storage and lookup
- **URL Slice**: Stores full URLs for exact matching
- **Regex Slice**: Stores compiled regex patterns

### Thread Safety

The package uses `sync.RWMutex` to ensure thread-safe operations.

### Performance Considerations

- Domain lookups are O(1) for exact matches
- Subdomain checks are O(n) where n is domain length
- Regex matching is the slowest operation

### Benchmarks

```
goos: darwin
goarch: arm64
pkg: github.com/internetarchive/Zeno/internal/pkg/postprocessor/domainscrawl
cpu: Apple M4
BenchmarkART_ExactMatch/N=1000_hit-10                   149810348                8.014 ns/op           0 B/op          0 allocs/op
BenchmarkART_ExactMatch/N=1000_miss-10                  188032132                6.379 ns/op           0 B/op          0 allocs/op
BenchmarkART_ExactMatch/N=10000_hit-10                  151031845                7.944 ns/op           0 B/op          0 allocs/op
BenchmarkART_ExactMatch/N=10000_miss-10                 186607285                6.429 ns/op           0 B/op          0 allocs/op
BenchmarkART_ExactMatch/N=50000_hit-10                  159776817                7.500 ns/op           0 B/op          0 allocs/op
BenchmarkART_ExactMatch/N=50000_miss-10                 164789222                7.233 ns/op           0 B/op          0 allocs/op
BenchmarkART_ExactMatch_Parallel-10                     12455469               118.0 ns/op             0 B/op          0 allocs/op
BenchmarkART_PrefixMatch/N=1000_hit-10                   8624710               143.3 ns/op           128 B/op          5 allocs/op
BenchmarkART_PrefixMatch/N=1000_miss-10                  7227160               139.8 ns/op           152 B/op          6 allocs/op
BenchmarkART_PrefixMatch/N=10000_hit-10                  6541926               175.6 ns/op           128 B/op          5 allocs/op
BenchmarkART_PrefixMatch/N=10000_miss-10                 7453480               160.7 ns/op           152 B/op          6 allocs/op
BenchmarkART_PrefixMatch/N=50000_hit-10                  7281529               163.5 ns/op           128 B/op          5 allocs/op
BenchmarkART_PrefixMatch/N=50000_miss-10                 7815578               151.4 ns/op           152 B/op          6 allocs/op
BenchmarkART_PrefixMatch_Parallel-10                     9661206               128.9 ns/op           136 B/op          5 allocs/op
BenchmarkMatch_NaiveDomain_Exact-10                     16274312                78.03 ns/op           96 B/op          1 allocs/op
BenchmarkMatch_NaiveDomain_Subdomain-10                  4647944               264.3 ns/op           224 B/op          6 allocs/op
BenchmarkMatch_NaiveDomain_Miss-10                       4879600               252.1 ns/op           248 B/op          7 allocs/op
BenchmarkMatch_NaiveDomain_Subdomain_Parallel-10         4748758               240.8 ns/op           232 B/op          6 allocs/op
BenchmarkMatch_FullURL_Exact-10                          4422847               280.4 ns/op           280 B/op          8 allocs/op
BenchmarkMatch_Regex-10                                   998388              1206 ns/op             517 B/op         16 allocs/op
BenchmarkReverseHost-10                                  1000000              1101 ns/op            1128 B/op         37 allocs/op
PASS
ok      github.com/internetarchive/Zeno/internal/pkg/postprocessor/domainscrawl 26.426s
```