# Zeno
State-of-the-art web crawler 🔱

## Introduction

Zeno is a web crawler designed to operate wide crawls or to simply archive one web page.
Zeno's key concepts are: portability, performance, simplicity.
With an emphasis on performance.

It heavily relies on the [gowarc](https://github.com/internetarchive/gowarc) module for traffic recording into [WARC](https://iipc.github.io/warc-specifications/) files.

The name Zeno comes from Zenodotus (Ζηνόδοτος), a Greek grammarian, literary critic, Homeric scholar,
and the first librarian of the Library of Alexandria.

## Requirements for Building
- **Go 1.25+** - As specified in go.mod
- If CGO_ENABLED=1:
   > **GCC 12+** - Required for building C++ dependencies with C++20 constexpr support for the WHATWG URL parser ([github.com/ada-url/goada](https://github.com/ada-url/goada)).
- If CGO_ENABLED=0:
   > No additional requirements, as the CGO-free WebAssembly wrapper of goada ([goada-wasm](https://github.com/yzqzss/goada-wasm/)) will be used. (slower 1x than CGO version on amd64 and arm64, slower **10x or more** on other CPU architectures! Check https://wazero.io/docs/#compiler for details)

Note: GCC 11 and earlier versions do not support the C++20 constexpr features required by the ada-url/goada dependency. On Ubuntu 22 LTS and earlier, you may need to install a newer GCC version or disable CGO.

## Installation

```bash
go install github.com/internetarchive/Zeno@latest
```

or utilize our pre-built [release binaries here](https://github.com/internetarchive/Zeno/releases), but do note that we are mainly focused on linux/amd64 support at this time.

## Quick Start

To archive a single web page:
```bash
Zeno get url https://www.france.fr
```

Zeno is highly configurable with many parameters that can be customized. To see all available configuration options, use `Zeno -h` and/or `Zeno get -h`.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request & open issues!

Zeno is being developed and maintained by the [Internet Archive](https://archive.org) and awesome contributors. The project has evolved into what it is today thanks to the invaluable contributions from the community. While we can't list everyone, special thanks to:

- [Corentin Barreau](https://github.com/CorentinB) former Wayback Machine Software Engineer at the [Internet Archive](https://archive.org) for his initial work on the project.
- [Jake LaFountain](https://github.com/NGTmeaty), Wayback Machine Software Engineer at the [Internet Archive](https://archive.org).
- [Thomas Foubert](https://github.com/equals215), former Wayback Machine Platform Engineer at the [Internet Archive](https://archive.org).
- [yzqzss](https://github.com/yzqzss), Lead Developer of the [Save The Web Project](https://github.com/saveweb).
- [Will Howes](https://github.com/willmhowes), Wayback Machine Software Engineer at the [Internet Archive](https://archive.org).
- [Vangelis Banos](https://github.com/vbanos), Wayback Machine Software Engineer at the [Internet Archive](https://archive.org).

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0). See the [LICENSE](LICENSE) file for details.
