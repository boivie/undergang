package main

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	ug "github.com/boivie/undergang/app"
	"github.com/codegangsta/cli"
	"io/ioutil"
	"net/http"
	"os"
)

func main() {
	log.SetFormatter(&log.TextFormatter{})
	log.SetLevel(log.DebugLevel)
	app := cli.NewApp()
	app.Name = "undergang"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "listen",
			Value: ":8002",
			Usage: "Listening address (ip and port)",
		},
		cli.StringFlag{
			Name:  "pathinfo",
			Usage: "URL for pathinfo service",
		},
		cli.StringFlag{
			Name:  "sshproxy",
			Usage: "Optional utility for proxying SSH connections",
		},
		cli.StringFlag{
			Name:  "config",
			Usage: "Configuration file",
		},
	}
	app.Action = func(c *cli.Context) {
		ug.Init(c.String("pathinfo"), c.String("access"), c.String("sshproxy"))

		if c.String("config") != "" {
			var config struct {
				Paths []ug.PathInfo
			}
			buf, err := ioutil.ReadFile(c.String("config"))
			if err != nil {
				panic(err)
			}
			if err = json.Unmarshal(buf, &config); err != nil {
				panic(err)
			}
			for _, path := range config.Paths {
				log.Infof("Adding path mapping \"%s\"", path.Prefix)
				ug.AddPath(path)
			}
		}

		log.Infof("Accepting requests on %s", c.String("listen"))
		err := http.ListenAndServe(c.String("listen"), nil)
		if err != nil {
			panic(err)
		}
	}

	app.Run(os.Args)
}
