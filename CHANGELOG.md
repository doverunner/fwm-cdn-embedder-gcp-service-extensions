# DoveRunner Forensic Watermark CDN Embedder — Changelog

All notable changes to the CDN embedder samples are documented in this file.

> - **Docs**: https://docs.doverunner.com
> - **Support**: https://doverunner.zendesk.com
> - **Copyright**: ⓒ 2026 DoveRunner. All Rights Reserved. (doverunner.com)

The Lambda@Edge, EdgeWorkers and Cloudflare Workers samples share one version from 3.0.0
onward. Earlier entries describe the Lambda@Edge sample.

---

## [3.0.0] - 2026-08

### Added
- Short-Form and Micro-Form watermark support

---

## [2.10.0] - 2026-03

### Added
- `syncSkipBit` option for controlling sync skip behavior

---

## [2.9.1] — 2026-02

### Fixed
- Block requests to `b.file` to prevent unauthorized file access

---

## [2.9.0] — 2025-08

### Added
- Session revoke feature

---

## [2.8.0] — 2023-05

### Added
- CMAF (Common Media Application Format) support

---

## [2.7.0] — 2023-04

### Fixed
- Resolved mismatch handling between `streaming_format` and file extension

---

## [2.6.0] — 2023-02

### Added
- Akamai WMT (Watermark Token) format support

---

## [2.5.1] — 2022-11

### Changed
- `config.prefixFolder` now accepts an array of values

---

## [2.5.0] — 2022-11

### Added
- `prefixFolder` configuration option for path prefix customization

---

## [2.1.0] — 2020-09

### Added
- `generate_tracktype_manifests` support

---

## [2.0.1] — 2020-03

### Changed
- Adjusted number of TS segments per GOP

---
