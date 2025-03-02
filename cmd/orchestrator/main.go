package main

import (
	"calculator-service/internal/orchestrator"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	port := os.Getenv("ORCHESTRATOR_PORT")
	if port == "" {
		port = "8080"
	}

	r := mux.NewRouter()

	r.HandleFunc("/api/v1/calculate", orchestrator.HandleCalculate).Methods("POST")
	r.HandleFunc("/api/v1/expressions", orchestrator.HandleGetExpressions).Methods("GET")
	r.HandleFunc("/api/v1/expressions/{id}", orchestrator.HandleGetExpression).Methods("GET")

	r.HandleFunc("/internal/task", orchestrator.HandleGetTask).Methods("GET")
	r.HandleFunc("/internal/task", orchestrator.HandleSubmitTaskResult).Methods("POST")

	webFS := http.FileServer(http.Dir("./cmd/web/static"))

	r.PathPrefix("/css/").Handler(http.StripPrefix("/css/", http.FileServer(http.Dir("./cmd/web/static/css"))))
	r.PathPrefix("/js/").Handler(http.StripPrefix("/js/", http.FileServer(http.Dir("./cmd/web/static/js"))))

	r.PathPrefix("/").Handler(webFS)

	log.Printf("Orchestrator starting on port %s", port)
	log.Printf("Web interface available at http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
