package main

import (
	"bytes"
	"calculator-service/internal/types"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const (
	orchestratorBaseURL = "http://localhost:8080"
)

func processTask(workerID int) {
	resp, err := http.Get(orchestratorBaseURL + "/internal/task")
	if err != nil {
		log.Printf("Worker %d: Error getting task: %v", workerID, err)
		time.Sleep(time.Second)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		time.Sleep(time.Second)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Worker %d: Unexpected status code: %d", workerID, resp.StatusCode)
		time.Sleep(time.Second)
		return
	}

	var task types.Task
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		log.Printf("Worker %d: Error decoding task: %v", workerID, err)
		return
	}

	result := calculateResult(task)

	taskResult := types.TaskResult{
		ID:     task.ID,
		Result: result,
	}

	resultJSON, err := json.Marshal(taskResult)
	if err != nil {
		log.Printf("Worker %d: Error marshaling result: %v", workerID, err)
		return
	}

	resp, err = http.Post(orchestratorBaseURL+"/internal/task", "application/json", bytes.NewBuffer(resultJSON))
	if err != nil {
		log.Printf("Worker %d: Error sending result: %v", workerID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Worker %d: Error response when sending result: %d", workerID, resp.StatusCode)
	}
}

func calculateResult(task types.Task) float64 {
	var delay time.Duration
	switch task.Operation {
	case "+":
		delay = time.Duration(TIME_ADDITION_MS) * time.Millisecond
	case "-":
		delay = time.Duration(TIME_SUBTRACTION_MS) * time.Millisecond
	case "*":
		delay = time.Duration(TIME_MULTIPLICATIONS_MS) * time.Millisecond
	case "/":
		delay = time.Duration(TIME_DIVISIONS_MS) * time.Millisecond
	}
	time.Sleep(delay)

	switch task.Operation {
	case "+":
		return task.Arg1 + task.Arg2
	case "-":
		return task.Arg1 - task.Arg2
	case "*":
		return task.Arg1 * task.Arg2
	case "/":
		if task.Arg2 == 0 {
			return 0
		}
		return task.Arg1 / task.Arg2
	default:
		return 0
	}
}
