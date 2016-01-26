package main;
import (
	"net/http"
	"github.com/boivie/undergang/app"
	"log"
)

func main() {
	app.Init("http://localhost:8001/_undergang/pathinfo")
	log.Println("Accepting requests")
	http.HandleFunc("/", app.Forward)
	err := http.ListenAndServe(":8002", nil)
	if err != nil {
		panic(err)
	}
}