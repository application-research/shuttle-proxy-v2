# Estuary Shuttle Proxy V2

## Installation
```
go mod tidy
go mod download
```

## Env
```
LISTEN_ADDR=0.0.0.0:8081
DB_NAME=
DB_HOST=
DB_USER=
DB_PASS=
DB_PORT=
```

## Setup
```
go build -tags netgo -ldflags '-s -w' -o shuttle-proxy
./shuttle-proxy
```
