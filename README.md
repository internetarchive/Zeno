# Zeno
State-of-the-art web crawler 🔱

## Introduction

Zeno is a web crawler designed to operate wide crawls or to simply archive one web page.
Zeno's key concepts are: portability, performance, simplicity.
With an emphasis on performance.

It heavily relies on the [warc](https://github.com/CorentinB/warc) module for traffic recording into [WARC](https://iipc.github.io/warc-specifications/) files.

The name Zeno comes from Zenodotus (Ζηνόδοτος), a Greek grammarian, literary critic, Homeric scholar,
and the first librarian of the Library of Alexandria.

## Installation

```bash
go install github.com/internetarchive/Zeno@latest
```

## Quick Start

To archive a single web page:
```bash
Zeno get url https://www.france.fr
```

Zeno is highly configurable with many parameters that can be customized. To see all available configuration options, use `Zeno -h` and/or `Zeno get -h`.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request & open issues!

Zeno is being developed and maintained by [Corentin Barreau](https://github.com/CorentinB) at the [Internet Archive](https://archive.org). The project has evolved into what it is today thanks to the invaluable contributions from the community. While we can't list everyone, special thanks to:

- [Jake LaFountain](https://github.com/NGTmeaty), Wayback Machine Software Engineer at the [Internet Archive](https://archive.org).
- [Thomas Foubert](https://github.com/equals215), Wayback Machine Platform Engineer at the [Internet Archive](https://archive.org).
- [yzqzss](https://github.com/yzqzss), Lead Developer of the [Save The Web Project](https://github.com/saveweb).

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0). See the [LICENSE](LICENSE) file for details.
