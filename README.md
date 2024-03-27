# Zeno
State-of-the-art web crawler 🔱

## Introduction

Zeno is a web crawler designed to operate wide crawls or to simply archive one web page.
Zeno's key concepts are: portability, performance, simplicity.
With an emphasis on performance.

It has been originally developed by [Corentin Barreau](https://github.com/CorentinB) at the Internet Archive.
It heavily relies on the [warc](https://github.com/CorentinB/warc) module for traffic recording into [WARC](https://iipc.github.io/warc-specifications/) files.

The name Zeno comes from Zenodotus (Ζηνόδοτος), a Greek grammarian, literary critic, Homeric scholar,
and the first librarian of the Library of Alexandria.

## Usage

See `./Zeno -h`

```
COMMANDS:
   get      Archive the web!
   version  Show the version number.
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --user-agent value                                     User agent to use when requesting URLs. (default: "Zeno")
   --job value                                            Job name to use, will determine the path for the persistent queue, seencheck database, and WARC files.
   --workers value, -w value                              Number of concurrent workers to run. (default: 1)
   --max-concurrent-assets value, --ca value              Max number of concurrent assets to fetch PER worker. E.g. if you have 100 workers and this setting at 8, Zeno could do up to 800 concurrent requests at any time. (default: 8)
   --max-hops value, --hops value                         Maximum number of hops to execute. (default: 0)
   --cookies value                                        File containing cookies that will be used for requests.
   --keep-cookies                                         Keep a global cookie jar (default: false)
   --headless                                             Use headless browsers instead of standard GET requests. (default: false)
   --local-seencheck                                      Simple local seencheck to avoid re-crawling of URIs. (default: false)
   --json                                                 Output logs in JSON (default: false)
   --debug                                                (default: false)
   --live-stats                                           (default: false)
   --api                                                  (default: false)
   --api-port value                                       Port to listen on for the API. (default: "9443")
   --prometheus                                           Export metrics in Prometheus format, using this setting imply --api. (default: false)
   --prometheus-prefix value                              String used as a prefix for the exported Prometheus metrics. (default: "zeno:")
   --max-redirect value                                   Specifies the maximum number of redirections to follow for a resource. (default: 20)
   --max-retry value                                      Number of retry if error happen when executing HTTP request. (default: 20)
   --http-timeout value                                   Number of seconds to wait before timing out a request. (default: 30)
   --domains-crawl                                        If this is turned on, seeds will be treated as domains to crawl, therefore same-domain outlinks will be added to the queue as hop=0. (default: false)
   --disable-html-tag value [ --disable-html-tag value ]  Specify HTML tag to not extract assets from
   --capture-alternate-pages                              If turned on, <link> HTML tags with "alternate" values for their "rel" attribute will be archived. (default: false)
   --exclude-host value [ --exclude-host value ]          Exclude a specific host from the crawl, note that it will not exclude the domain if it is encountered as an asset for another web page.
   --max-concurrent-per-domain value                      Maximum number of concurrent requests per domain. (default: 16)
   --concurrent-sleep-length value                        Number of milliseconds to sleep when max concurrency per domain is reached. (default: 500)
   --crawl-time-limit value                               Number of seconds until the crawl will automatically set itself into the finished state. (default: 0)
   --crawl-max-time-limit value                           Number of seconds until the crawl will automatically panic itself. Default to crawl-time-limit + (crawl-time-limit / 10) (default: 0)
   --proxy value                                          Proxy to use when requesting pages.
   --bypass-proxy value [ --bypass-proxy value ]          Domains that should not be proxied.
   --warc-prefix value                                    Prefix to use when naming the WARC files. (default: "ZENO")
   --warc-operator value                                  Contact informations of the crawl operator to write in the Warc-Info record in each WARC file.
   --warc-cdx-dedupe-server value                         Identify the server to use CDX deduplication. This also activates CDX deduplication on.
   --warc-on-disk                                         Do not use RAM to store payloads when recording traffic to WARCs, everything will happen on disk (usually used to reduce memory usage). (default: false)
   --warc-pool-size value                                 Number of concurrent WARC files to write. (default: 1)
   --warc-temp-dir value                                  Custom directory to use for WARC temporary files.
   --disable-local-dedupe                                 Disable local URL agonistic deduplication. (default: false)
   --cert-validation                                      Enables certificate validation on HTTPS requests. (default: false)
   --disable-assets-capture                               Disable assets capture. (default: false)
   --warc-dedupe-size value                               Minimum size to deduplicate WARC records with revisit records. (default: 1024)
   --cdx-cookie value                                     Pass custom cookie during CDX requests. Example: 'cdx_auth_token=test_value'
   --hq                                                   Use Crawl HQ to pull URLs to process. (default: false)
   --hq-address value                                     Crawl HQ address.
   --hq-key value                                         Crawl HQ key.
   --hq-secret value                                      Crawl HQ secret.
   --hq-project value                                     Crawl HQ project.
   --hq-batch-size value                                  Crawl HQ feeding batch size. (default: 0)
   --hq-continuous-pull                                   If turned on, the crawler will pull URLs from Crawl HQ continuously. (default: false)
   --hq-strategy value                                    Crawl HQ feeding strategy. (default: "lifo")
   --es-url value                                         ElasticSearch URL to use for indexing crawl logs.
   --exclude-string value [ --exclude-string value ]      Discard any (discovered) URLs containing this string.
   --help, -h                                             show help
   --version, -v                                          print the version
   ```