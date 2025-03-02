package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func waitForOrchestrator(timeout time.Duration) bool {
	start := time.Now()
	for {
		if time.Since(start) > timeout {
			return false
		}

		resp, err := http.Get("http://localhost:8080/api/v1/expressions")
		if err == nil {
			resp.Body.Close()
			return true
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func main() {
	goCmd := "go"

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	wg.Add(2)

	done := make(chan struct{})

	fmt.Println("Запуск оркестратора...")
	orchestratorCmd := exec.Command(goCmd, "run", "./cmd/orchestrator")
	orchestratorCmd.Stdout = os.Stdout
	orchestratorCmd.Stderr = os.Stderr

	err := orchestratorCmd.Start()
	if err != nil {
		log.Fatalf("Ошибка запуска оркестратора: %v", err)
	}

	fmt.Println("Ожидание готовности оркестратора...")
	if !waitForOrchestrator(15 * time.Second) {
		orchestratorCmd.Process.Kill()
		log.Fatalf("Превышено время ожидания запуска оркестратора")
	}

	fmt.Println("Оркестратор готов. Запуск агента...")
	agentCmd := exec.Command(goCmd, "run", "./cmd/agent")
	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr

	err = agentCmd.Start()
	if err != nil {
		orchestratorCmd.Process.Kill()
		log.Fatalf("Ошибка запуска агента: %v", err)
	}

	go func() {
		<-sigs
		fmt.Println("\nПолучен сигнал завершения. Завершаем процессы...")

		if agentCmd.Process != nil {
			agentCmd.Process.Kill()
		}

		if orchestratorCmd.Process != nil {
			orchestratorCmd.Process.Kill()
		}

		close(done)
	}()

	go func() {
		defer wg.Done()
		err = orchestratorCmd.Wait()
		if err != nil {
			fmt.Printf("Оркестратор завершился с ошибкой: %v\n", err)
		} else {
			fmt.Println("Оркестратор успешно завершил работу")
		}
	}()

	go func() {
		defer wg.Done()
		err = agentCmd.Wait()
		if err != nil {
			fmt.Printf("Агент завершился с ошибкой: %v\n", err)
		} else {
			fmt.Println("Агент успешно завершил работу")
		}
	}()

	go func() {
		wg.Wait()
		close(done)
	}()

	<-done
	fmt.Println("Все процессы завершены.")
}
