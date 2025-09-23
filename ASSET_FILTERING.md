# Asset Filtering Features

This document demonstrates the new asset filtering features implemented for Zeno.

## New Command-Line Flags

### 1. Maximum Assets per Page (`--max-assets`)
Limits the number of assets archived per page.

```bash
# Archive at most 5 assets per page
./Zeno get url https://example.com --max-assets 5
```

### 2. File Type Filtering

#### Allowed File Types (`--assets-allowed-file-types`)
Only archive assets with specified file extensions.

```bash
# Only archive CSS and JavaScript files
./Zeno get url https://example.com --assets-allowed-file-types css,js

# Only archive images
./Zeno get url https://example.com --assets-allowed-file-types jpg,png,gif,webp
```

#### Disallowed File Types (`--assets-disallowed-file-types`)
Exclude specific file extensions from archiving.

```bash
# Archive everything except video files
./Zeno get url https://example.com --assets-disallowed-file-types mp4,avi,mov,mkv

# Exclude large file types
./Zeno get url https://example.com --assets-disallowed-file-types mp4,zip,tar,gz
```

**Note:** If both `--assets-allowed-file-types` and `--assets-disallowed-file-types` are specified, the allowed types take precedence and disallowed types are ignored.

### 3. Time-Based Filtering (`--assets-archiving-timeout`)
Stop archiving assets after a specified time per page.

```bash
# Stop archiving assets after 30 seconds per page
./Zeno get url https://example.com --assets-archiving-timeout 30s

# Stop after 2 minutes
./Zeno get url https://example.com --assets-archiving-timeout 2m
```

## Combined Usage

All filters can be combined for fine-grained control:

```bash
# Archive at most 10 CSS/JS files, with 1 minute timeout per page
./Zeno get url https://example.com \
  --max-assets 10 \
  --assets-allowed-file-types css,js \
  --assets-archiving-timeout 1m
```

## Default Behavior

When no asset filtering flags are specified, Zeno maintains its existing behavior of archiving all assets found on each page.

## Implementation Details

- **File type filtering** is applied during asset extraction in the postprocessor
- **Maximum assets limit** is applied after file type filtering
- **Timeout-based filtering** is applied during the archiving phase using Go context cancellation
- Assets that are skipped due to timeout have their status set to completed to avoid reprocessing
- All filtering preserves the existing behavior when no filters are configured

## Testing

The implementation includes comprehensive tests covering:
- Individual filter functionality
- Combined filter scenarios  
- Edge cases (nil assets, invalid URLs)
- Integration with the existing asset extraction pipeline