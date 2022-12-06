package main

import (
	"fmt"
	logging "github.com/ipfs/go-log/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
	"github.com/vulcand/oxy/buffer"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/roundrobin"
	"github.com/vulcand/oxy/testutils"
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
	Forwarder     *forward.Forwarder
	Server        *http.Server
}

func main() {
	logging.SetLogLevel("shuttle-forwarder", "debug")

	app := cli.NewApp()
	app.Version = appVersion

	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "reshuffle",
			Aliases: []string{"r"},
			Usage:   "reshuffle the endpoints",
			Action: func(context *cli.Context, s bool) error {
				reshuffle()
				return nil
			},
		},
	}

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
		fwd, _ := forward.New()
		lb, _ := roundrobin.New(fwd)

		proxy.Forwarder = fwd
		proxy.LoadBalancer = lb

		// 	get preferred endpoints
		endpoints := proxy.getPreferredEndpoints()
		proxy.Endpoints = endpoints

		for _, endpoint := range endpoints {
			lb.UpsertServer(testutils.ParseURI("https://" + endpoint))
			fmt.Println(endpoint)
		}

		//// additional rebalancer logic
		rb, err := roundrobin.NewRebalancer(lb,
			roundrobin.RebalancerRequestRewriteListener(func(oldReq *http.Request, newReq *http.Request) {
			}))

		buffer, err := buffer.New(rb,
			buffer.Retry(`IsNetworkError() && Attempts() <= 2`),
			buffer.Retry(`ResponseCode() == 400 && Attempts() <= 2`),
			buffer.Retry(`ResponseCode() == 404 && Attempts() <= 2`),
			buffer.Retry(`ResponseCode() == 502 && Attempts() <= 2`),
			buffer.Retry(`ResponseCode() == 504 && Attempts() <= 2`))
		if err != nil {
			panic(err)
		}

		s := &http.Server{
			Addr:    viper.GetString("LISTEN_ADDR"),
			Handler: buffer,
		}

		e.Server = s
		proxy.Server = s
		return s.ListenAndServe()
	}

	app.RunAndExitOnError()
}

func reshuffle() {
	endpoints := proxy.getPreferredEndpoints()
	proxy.Endpoints = endpoints

	fwd, _ := forward.New()
	lb, _ := roundrobin.New(fwd)

	for _, endpoint := range endpoints {
		lb.UpsertServer(testutils.ParseURI(endpoint))
	}

	proxy.Forwarder = fwd
	proxy.LoadBalancer = lb
	proxy.Server.Handler = lb

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
