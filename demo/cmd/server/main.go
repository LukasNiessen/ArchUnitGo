// Command server wires the hexagon together: the in-memory adapter into the use
// case, the use case into the HTTP adapter, and the result into a mux. The
// dependency direction here is the whole point of the architecture tests.
package main

import (
	"log"
	"net/http"

	httpadapter "github.com/LukasNiessen/ArchUnitGoDemo/internal/adapters/primary/http"
	"github.com/LukasNiessen/ArchUnitGoDemo/internal/adapters/secondary/memory"
	"github.com/LukasNiessen/ArchUnitGoDemo/internal/application"
)

func main() {
	orders := memory.NewOrderRepository()
	placeOrder := application.PlaceOrder{Orders: orders}
	handler := httpadapter.Handler{PlaceOrder: placeOrder}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", handler.Place)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
