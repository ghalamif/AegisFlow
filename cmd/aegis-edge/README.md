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

The startup banner lives in `assets/banner_color.ansi` (ANSI colors) and `assets/banner_plain.txt` (fallback). To preview the colored version locally:

```
cat cmd/aegis-edge/assets/banner_color.ansi
```

Most Markdown renderers cannot display the escape codes inline, so view the file in a terminal to see the actual colors.
