package main;
import (
	"net/http"
	"github.com/boivie/undergang/app"
	"log"
)


func main() {
	app.Init()
	http.HandleFunc("/", app.Forward)

	log.Println("Accepting requests")
	http.ListenAndServe(":8000", nil)
}