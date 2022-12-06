# Estuary Shuttle Proxy V2

A smarter proxy for Estuary shuttles.

It does a round robin check of the shuttles and retries the request on other shuttles if the chosen is down.

## Installation
```
go mod tidy
go mod download
```

## Setup
```
go build -tags netgo -ldflags '-s -w' -o shuttle-proxy
./shuttle-proxy
```

## Live service
```

```