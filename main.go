package main

import (
	"fmt"
	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
	"github.com/vulcand/oxy/testutils"
	"github.com/vulcand/oxy/v2/buffer"
	"github.com/vulcand/oxy/v2/forward"
	"github.com/vulcand/oxy/v2/roundrobin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"net/http"
)

var appVersion string

var log = logging.Logger("shuttle-proxy").With("app_version", appVersion)
var proxy *Proxy
var proxies []string

type Proxy struct {
	ControllerUrl string
	DB            *gorm.DB
	Endpoints     []string
	LoadBalancer  *roundrobin.RoundRobin
	Forwarder     *forward.URLForwardingStateListener
	Server        *http.Server
}

func main() {
	logging.SetLogLevel("shuttle-forwarder", "debug")

	app := cli.NewApp()
	app.Version = appVersion
	app.Action = func(cctx *cli.Context) error {
		log.Infof("shuttle proxy version: %s", appVersion)

		logging := cctx.Bool("logging")

		database, err := setupDB()
		proxy = &Proxy{
			ControllerUrl: cctx.String("controller"),
			DB:            database,
		}

		// upload.estuary.tech
		// routes to whatever shuttle is best
		// this is entirely because you cant return a 302 to a file upload request
		e := echo.New()
		e.HideBanner = true
		if logging {
			e.Use(middleware.Logger())
		}

		e.Use(middleware.CORS())

		// Forwards incoming requests to whatever location URL points to, adds proper forwarding headers
		fwd := forward.New(false)
		lb, _ := roundrobin.New(fwd)

		proxy.LoadBalancer = lb

		buf, err := buffer.New(lb,
			buffer.Retry(`IsNetworkError() && Attempts() < 2`),
			buffer.Retry(`Attempts() < 2 && ResponseCode() == 400`),
			buffer.Retry(`Attempts() < 2 && ResponseCode() == 404`),
			buffer.Retry(`Attempts() < 2 && ResponseCode() == 500`),
			buffer.Retry(`Attempts() < 2 && ResponseCode() == 502`))

		// 	get preferred endpoints
		endpoints := proxy.getPreferredEndpoints()
		proxy.Endpoints = endpoints

		for _, endpoint := range endpoints {
			lb.UpsertServer(testutils.ParseURI("https://" + endpoint))
			fmt.Println(endpoint)
		}

		if err != nil {
			panic(err)
		}

		s := &http.Server{
			Addr:    viper.GetString("LISTEN_ADDR"),
			Handler: buf,
		}

		e.Server = s
		proxy.Server = s
		return s.ListenAndServe()
	}

	app.RunAndExitOnError()
}

func (p *Proxy) getPreferredEndpoints() []string {
	// select host from shuttles where open = true;
	p.DB.Raw("select host from shuttles where open = true").Scan(&proxies)
	return proxies
}

func setupDB() (*gorm.DB, error) { // it's a pointer to a gorm.DB

	viper.SetConfigFile(".env")
	err := viper.ReadInConfig()

	dbHost, okHost := viper.Get("DB_HOST").(string)
	dbUser, okUser := viper.Get("DB_USER").(string)
	dbPass, okPass := viper.Get("DB_PASS").(string)
	dbName, okName := viper.Get("DB_NAME").(string)
	dbPort, okPort := viper.Get("DB_PORT").(string)
	if !okHost || !okUser || !okPass || !okName || !okPort {
		panic("invalid database configuration")
	}

	dsn := "host=" + dbHost + " user=" + dbUser + " password=" + dbPass + " dbname=" + dbName + " port=" + dbPort + " sslmode=disable TimeZone=Asia/Shanghai"

	DB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return DB, nil
}
