# Estuary Shuttle Proxy V2

A smarter proxy for Estuary shuttles.

It does a roundrobin check of the shuttles and retries the request on other shuttles if the chosen is down.

## Installation
```
go mod tidy
go mod download
```

## Env (create a `.env` file)
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

