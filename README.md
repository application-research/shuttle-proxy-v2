# Estuary Shuttle Proxy V2

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
