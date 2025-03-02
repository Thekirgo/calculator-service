package orchestrator

import (
	"calculator-service/internal/calculator"
	"calculator-service/internal/types"
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

var (
	expressions      = make(map[string]types.Expression)
	tasks            = make(map[string]types.Task)
	taskResults      = make(map[string]float64)
	taskToExpression = make(map[string]string)
	expressionTasks  = make(map[string][]string)
	dependsOnTask    = make(map[string]string) // Карта зависимостей: taskID -> taskID, от которого зависит
	mu               sync.RWMutex
	calc             = calculator.NewCalculator()
)

type stackItem struct {
	value  float64
	taskID string
	isNum  bool
}

func ResetState() {
	mu.Lock()
	defer mu.Unlock()

	expressions = make(map[string]types.Expression)
	tasks = make(map[string]types.Task)
	taskResults = make(map[string]float64)
	taskToExpression = make(map[string]string)
	expressionTasks = make(map[string][]string)
	dependsOnTask = make(map[string]string)
	calc = calculator.NewCalculator()
}

func HandleCalculate(w http.ResponseWriter, r *http.Request) {
	var req types.CalculateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	exprID := uuid.New().String()
	expr := types.Expression{
		ID:       exprID,
		Original: req.Expression,
		Status:   "PROCESSING",
	}

	testCalc := calculator.NewCalculator()
	calculatedResult, err := testCalc.Calculate(req.Expression)
	if err != nil {
		if strings.Contains(err.Error(), "division by zero") ||
			strings.Contains(err.Error(), "invalid character") ||
			strings.Contains(err.Error(), "mismatched parentheses") ||
			strings.Contains(err.Error(), "invalid expression") ||
			strings.Contains(err.Error(), "empty expression") {
			http.Error(w, "Invalid expression: "+err.Error(), http.StatusUnprocessableEntity) // 422
			return
		}

		http.Error(w, "Error processing expression: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := calc.Tokenize(req.Expression); err != nil {
		http.Error(w, "Invalid expression: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	rpn, err := calc.ToRPN()
	if err != nil {
		http.Error(w, "Invalid expression: "+err.Error(), http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if len(rpn) == 1 && rpn[0].Type == calculator.Number {
		expr.Status = "COMPLETED"
		expr.Result = calculatedResult
		expressions[exprID] = expr

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": exprID})
		return
	}

	expressions[exprID] = expr

	var taskIDs []string
	var stack []stackItem

	for _, token := range rpn {
		switch token.Type {
		case calculator.Number:
			num, _ := strconv.ParseFloat(token.Value, 64)
			stack = append(stack, stackItem{
				value: num,
				isNum: true,
			})
		case calculator.Operator:
			if len(stack) < 2 {
				http.Error(w, "Invalid expression", http.StatusBadRequest)
				return
			}

			taskID := uuid.New().String()

			rightOp := stack[len(stack)-1]
			leftOp := stack[len(stack)-2]
			stack = stack[:len(stack)-2]

			task := types.Task{
				ID:        taskID,
				Operation: token.Value,
			}

			if token.Value == "*" || token.Value == "/" {
				task.Priority = 2
			} else {
				task.Priority = 1
			}

			if leftOp.isNum {
				task.Arg1 = leftOp.value
			} else {
				task.Arg1 = 0
				dependsOnTask[taskID] = leftOp.taskID
			}

			if rightOp.isNum {
				task.Arg2 = rightOp.value
			} else {
				task.Arg2 = 0
				if _, exists := dependsOnTask[taskID]; !exists {
					dependsOnTask[taskID] = rightOp.taskID
				}
			}

			tasks[taskID] = task
			taskToExpression[taskID] = exprID
			taskIDs = append(taskIDs, taskID)

			stack = append(stack, stackItem{
				taskID: taskID,
				isNum:  false,
			})
		}
	}

	expressionTasks[exprID] = taskIDs

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": exprID})
}

func HandleGetExpressions(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	var expressionsList []types.Expression
	for _, expr := range expressions {
		expressionsList = append(expressionsList, expr)
	}

	sort.Slice(expressionsList, func(i, j int) bool {
		return expressionsList[i].ID < expressionsList[j].ID
	})

	response := types.ExpressionResponse{
		Expressions: expressionsList,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func HandleGetExpression(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	mu.RLock()
	expr, exists := expressions[id]
	mu.RUnlock()

	if !exists {
		http.Error(w, "Expression not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(expr)
}

func HandleGetTask(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	for id, task := range tasks {
		if task.Priority == 2 {
			dependTaskID, hasDependency := dependsOnTask[id]
			if !hasDependency {
				delete(tasks, id)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(task)
				return
			}

			if result, ok := taskResults[dependTaskID]; ok {
				if task.Arg1 == 0 {
					task.Arg1 = result
				} else {
					task.Arg2 = result
				}

				delete(tasks, id)
				delete(dependsOnTask, id)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(task)
				return
			}
		}
	}

	for id, task := range tasks {
		if task.Priority == 1 {
			dependTaskID, hasDependency := dependsOnTask[id]
			if !hasDependency {
				delete(tasks, id)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(task)
				return
			}

			if result, ok := taskResults[dependTaskID]; ok {
				if task.Arg1 == 0 {
					task.Arg1 = result
				} else {
					task.Arg2 = result
				}

				delete(tasks, id)
				delete(dependsOnTask, id)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(task)
				return
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func HandleSubmitTaskResult(w http.ResponseWriter, r *http.Request) {
	var result types.TaskResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	taskResults[result.ID] = result.Result

	exprID, exists := taskToExpression[result.ID]
	if !exists {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	taskIDs, ok := expressionTasks[exprID]
	if !ok {
		http.Error(w, "Expression tasks not found", http.StatusNotFound)
		return
	}

	allTasksCompleted := true
	for _, taskID := range taskIDs {
		if _, ok := taskResults[taskID]; !ok {
			allTasksCompleted = false
			break
		}
	}

	if allTasksCompleted {
		expr := expressions[exprID]
		finalResult, err := calculator.Calc(expr.Original)
		if err != nil {
			expr.Status = "ERROR"
			expressions[exprID] = expr
			log.Printf("Error calculating expression %s: %v", expr.Original, err)
		} else {
			expr.Status = "COMPLETED"
			expr.Result = finalResult
			expressions[exprID] = expr
		}

		for _, taskID := range taskIDs {
			delete(taskResults, taskID)
			delete(taskToExpression, taskID)
			delete(dependsOnTask, taskID)
			delete(tasks, taskID)
		}
		delete(expressionTasks, exprID)
	}

	w.WriteHeader(http.StatusOK)
}
