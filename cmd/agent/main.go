package main

import (
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
)

var (
	TIME_ADDITION_MS        int
	TIME_SUBTRACTION_MS     int
	TIME_MULTIPLICATIONS_MS int
	TIME_DIVISIONS_MS       int
	COMPUTING_POWER         int
)

func loadConfig() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	var err error
	TIME_ADDITION_MS, err = strconv.Atoi(getEnvOrDefault("TIME_ADDITION_MS", "1000"))
	if err != nil {
		log.Fatal("Invalid TIME_ADDITION_MS")
	}

	TIME_SUBTRACTION_MS, err = strconv.Atoi(getEnvOrDefault("TIME_SUBTRACTION_MS", "1000"))
	if err != nil {
		log.Fatal("Invalid TIME_SUBTRACTION_MS")
	}

	TIME_MULTIPLICATIONS_MS, err = strconv.Atoi(getEnvOrDefault("TIME_MULTIPLICATIONS_MS", "1000"))
	if err != nil {
		log.Fatal("Invalid TIME_MULTIPLICATIONS_MS")
	}

	TIME_DIVISIONS_MS, err = strconv.Atoi(getEnvOrDefault("TIME_DIVISIONS_MS", "1000"))
	if err != nil {
		log.Fatal("Invalid TIME_DIVISIONS_MS")
	}

	COMPUTING_POWER, err = strconv.Atoi(getEnvOrDefault("COMPUTING_POWER", "4"))
	if err != nil {
		log.Fatal("Invalid COMPUTING_POWER")
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	loadConfig()

	sem := make(chan struct{}, COMPUTING_POWER)
	var wg sync.WaitGroup

	log.Printf("Agent started with computing power: %d", COMPUTING_POWER)

	for i := 0; i < COMPUTING_POWER; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				sem <- struct{}{}
				processTask(workerID)
				<-sem
			}
		}(i)
	}

	wg.Wait()
}
