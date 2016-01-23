package main;
import (
	"net/http"
	"github.com/boivie/ssht/ssht"
	"log"
)


func main() {
	ssht.Init()
	http.HandleFunc("/", ssht.Forward)

	log.Println("Accepting requests")
	http.ListenAndServe(":8000", nil)
}