# Headless/Headfull Archiver

`behaviors.js` builds from the [browsertrix-behaviors](https://github.com/webrecorder/browsertrix-behaviors) v0.9.0 (AGPL-3.0).

## Differences from general archiver

- All assets extractors are disabled

## Timeouts

```mermaid
gantt
    title Global timeout: --headless-page-timeout
    dateFormat  YYYY-MM-DD HH:mm
    axisFormat  %H:%M

    section Page Load & Delays
    Wait for Page Load (--headless-page-load-timeout) :a2, 2024-06-01 10:00, 30m
    Post-Load Delay (--headless-page-post-load-delay) :a3, after a2, 3m

    section Behavior Execution
    Run Behavior Script (--headless-behavior-timeout) :a4, after a3, 30m

    section Request Wait & Archiving
    Wait for In-flight Requests :a5, after a4, 5m
    Extract and Store HTML      :a6, after a5, 1m
```

---

TODOs:

- [ ] Retry on bad status codes
