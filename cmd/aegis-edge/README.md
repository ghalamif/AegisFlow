# aegis-edge CLI

## Overview

`aegis-edge` launches the AegisFlow edge runtime, validates configs, and streams live metrics.

## Installation

```
go install github.com/ghalamif/AegisFlow/cmd/aegis-edge@latest
```

## Commands

```
aegis-edge run -config ./data/config.yaml
aegis-edge validate -config ./data/config.yaml
aegis-edge stats -url http://localhost:9100/metrics -interval 1s
```

## Banner Artwork

![CLI Banner](assets/banner_preview.png)

The ANSI sources live in `assets/banner_color.ansi` (color) and `assets/banner_plain.txt` (fallback). Preview them in a terminal to see the live colors:

```
cat cmd/aegis-edge/assets/banner_color.ansi
```
