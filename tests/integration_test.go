package tests

import (
	"calculator-service/internal/calculator"
	"calculator-service/internal/types"
	"fmt"
	"sync"
	"testing"
	"time"
)

type TestOrchestrator struct {
	expressions      map[string]types.Expression
	tasks            map[string]types.Task
	taskResults      map[string]float64
	taskToExpression map[string]string
	expressionTasks  map[string][]string
	dependsOnTask    map[string]string
	mu               sync.Mutex
	calc             *calculator.Calculator
	completedTasks   int
}

func NewTestOrchestrator() *TestOrchestrator {
	return &TestOrchestrator{
		expressions:      make(map[string]types.Expression),
		tasks:            make(map[string]types.Task),
		taskResults:      make(map[string]float64),
		taskToExpression: make(map[string]string),
		expressionTasks:  make(map[string][]string),
		dependsOnTask:    make(map[string]string),
		calc:             calculator.NewCalculator(),
	}
}

func (o *TestOrchestrator) Calculate(expression string) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	_, err := calculator.Calc(expression)
	if err != nil {
		return "", err
	}

	exprID := fmt.Sprintf("expr-%d", len(o.expressions)+1)
	expr := types.Expression{
		ID:       exprID,
		Original: expression,
		Status:   "PROCESSING",
	}

	if err := o.calc.Tokenize(expression); err != nil {
		return "", err
	}

	rpn, err := o.calc.ToRPN()
	if err != nil {
		return "", err
	}

	var taskIDs []string
	for i, token := range rpn {
		if token.Type == calculator.Operator {
			taskID := fmt.Sprintf("task-%s-%d", exprID, i)
			priority := 1
			if token.Value == "*" || token.Value == "/" {
				priority = 2
			}

			task := types.Task{
				ID:        taskID,
				Operation: token.Value,
				Priority:  priority,
			}

			o.tasks[taskID] = task
			taskIDs = append(taskIDs, taskID)
			o.taskToExpression[taskID] = exprID
		}
	}

	o.expressions[exprID] = expr
	o.expressionTasks[exprID] = taskIDs

	return exprID, nil
}

func (o *TestOrchestrator) GetTask() (*types.Task, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for id, task := range o.tasks {
		if task.Priority == 2 {
			delete(o.tasks, id)
			return &task, true
		}
	}

	for id, task := range o.tasks {
		if task.Priority == 1 {
			delete(o.tasks, id)
			return &task, true
		}
	}

	return nil, false
}

func (o *TestOrchestrator) SubmitTaskResult(result types.TaskResult) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.taskResults[result.ID] = result.Result
	o.completedTasks++

	exprID, exists := o.taskToExpression[result.ID]
	if !exists {
		return fmt.Errorf("task not found")
	}

	taskIDs, ok := o.expressionTasks[exprID]
	if !ok {
		return fmt.Errorf("expression tasks not found")
	}

	allTasksCompleted := true
	for _, taskID := range taskIDs {
		if _, ok := o.taskResults[taskID]; !ok {
			allTasksCompleted = false
			break
		}
	}

	if allTasksCompleted {
		expr := o.expressions[exprID]
		finalResult, err := calculator.Calc(expr.Original)
		if err != nil {
			expr.Status = "ERROR"
		} else {
			expr.Status = "COMPLETED"
			expr.Result = finalResult
		}
		o.expressions[exprID] = expr
	}

	return nil
}

func (o *TestOrchestrator) GetExpression(id string) (types.Expression, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	expr, exists := o.expressions[id]
	return expr, exists
}

func (o *TestOrchestrator) RunAgent(t *testing.T, wg *sync.WaitGroup, id int) {
	defer wg.Done()

	for i := 0; i < 10; i++ {
		task, found := o.GetTask()
		if !found {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		var result float64
		switch task.Operation {
		case "+":
			result = task.Arg1 + task.Arg2
		case "-":
			result = task.Arg1 - task.Arg2
		case "*":
			result = task.Arg1 * task.Arg2
		case "/":
			if task.Arg2 == 0 {
				result = 0
			} else {
				result = task.Arg1 / task.Arg2
			}
		}

		taskResult := types.TaskResult{
			ID:     task.ID,
			Result: result,
		}
		err := o.SubmitTaskResult(taskResult)
		if err != nil {
			t.Logf("Агент %d: ошибка отправки результата: %v", id, err)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

func TestFullExpressionCalculation(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		expected   float64
	}{
		{
			name:       "простое сложение",
			expression: "2+2",
			expected:   4,
		},
		{
			name:       "выражение с приоритетом",
			expression: "2+2*2",
			expected:   6,
		},
		{
			name:       "сложное выражение",
			expression: "5*(3+2)/5-1",
			expected:   4,
		},
		{
			name:       "выражение с несколькими операциями",
			expression: "10-2*3+4/2",
			expected:   6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orch := NewTestOrchestrator()

			exprID, err := orch.Calculate(tt.expression)
			if err != nil {
				t.Fatalf("Ошибка при отправке выражения: %v", err)
			}

			var wg sync.WaitGroup
			for i := 0; i < 3; i++ {
				wg.Add(1)
				go orch.RunAgent(t, &wg, i)
			}

			wg.Wait()
			expr, exists := orch.GetExpression(exprID)
			if !exists {
				t.Fatalf("Выражение не найдено")
			}

			if expr.Status != "COMPLETED" {
				t.Errorf("Статус выражения = %s, ожидается COMPLETED", expr.Status)
			}

			if expr.Result != tt.expected {
				t.Errorf("Результат выражения = %f, ожидается %f", expr.Result, tt.expected)
			}
		})
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		expectErr  bool
	}{
		{
			name:       "деление на ноль",
			expression: "5/0",
			expectErr:  true,
		},
		{
			name:       "некорректный символ",
			expression: "2+a",
			expectErr:  true,
		},
		{
			name:       "несоответствие скобок",
			expression: "(2+3",
			expectErr:  true,
		},
		{
			name:       "незавершенное выражение",
			expression: "1+",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orch := NewTestOrchestrator()

			_, err := orch.Calculate(tt.expression)

			if (err != nil) != tt.expectErr {
				t.Errorf("Calculate() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestConcurrentExpressionProcessing(t *testing.T) {
	orch := NewTestOrchestrator()
	expressions := []string{
		"2+3*4",   // 14
		"10/2+5",  // 10
		"7-2+3*2", // 11
		"20/4*2",  // 10
	}
	expected := []float64{14, 10, 11, 10}

	var exprIDs []string
	var wg sync.WaitGroup

	for _, expr := range expressions {
		exprID, err := orch.Calculate(expr)
		if err != nil {
			t.Fatalf("Ошибка при отправке выражения: %v", err)
		}
		exprIDs = append(exprIDs, exprID)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go orch.RunAgent(t, &wg, i)
	}

	wg.Wait()

	for i, exprID := range exprIDs {
		expr, exists := orch.GetExpression(exprID)
		if !exists {
			t.Fatalf("Выражение %s не найдено", exprID)
		}

		if expr.Status != "COMPLETED" {
			t.Errorf("Статус выражения %s = %s, ожидается COMPLETED", exprID, expr.Status)
		}

		if expr.Result != expected[i] {
			t.Errorf("Результат выражения %s = %f, ожидается %f", exprID, expr.Result, expected[i])
		}
	}
}
