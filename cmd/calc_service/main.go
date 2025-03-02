package main

import (
	"calculator-service/internal/api"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := mux.NewRouter()
	apiV1 := r.PathPrefix("/api/v1").Subrouter()
	handler := api.NewCalculatorHandler()
	apiV1.HandleFunc("/calculate", handler.Calculate).Methods(http.MethodPost)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
