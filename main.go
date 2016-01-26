package main;
import (
	"net/http"
	"github.com/boivie/undergang/app"
	"log"
	"io/ioutil"
)

func main() {
	key, err := ioutil.ReadFile("go_id_rsa")
	if err != nil {
		panic(err)
	}

	app.Init()

	app.AddPath(app.PathInfo{
		Prefix: "/pi/helloworld/",
		Server: app.ServerInfo{
			Address: "192.168.1.150:22",
			Username: "victor",
			SSHKey: string(key),
		},
		HttpProxy: &app.HttpProxy{
			Address: "127.0.0.1:8899",
			BasePath: "/",
		},
	})

	app.AddPath(app.PathInfo{
		Prefix: "/pi/lovebeat/",
		Server: app.ServerInfo{
			Address: "192.168.1.150:22",
			Username: "victor",
			SSHKey: string(key),
		},
		HttpProxy: &app.HttpProxy{
			Address: "127.0.0.1:8080",
			BasePath: "/",
		},
	})

	app.AddPath(app.PathInfo{
		Prefix: "/fremen/bash/",
		Server: app.ServerInfo{
			Address: "1.1.1.1:22",
			Username: "victor",
			SSHKey: string(key),
			Bootstrap: []string{
				"/bin/sh -c '/usr/bin/curl -L https://github.com/yudai/gotty/releases/download/v0.0.12/gotty_linux_arm.tar.gz | /bin/tar zxv ./gotty -O - > /tmp/gotty'",
				"/bin/chmod a+x /tmp/gotty",
			},
			Run: "/usr/bin/nohup /tmp/gotty -p 8083 -w -a 127.0.0.1 /bin/bash",
		},
		HttpProxy: &app.HttpProxy{
			Address: "127.0.0.1:8083",
			BasePath: "/",
		},
	})

	log.Println("Accepting requests")
	http.HandleFunc("/", app.Forward)
	http.ListenAndServe(":8000", nil)
}