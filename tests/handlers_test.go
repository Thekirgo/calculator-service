package tests

import (
	"bytes"
	"calculator-service/internal/calculator"
	"calculator-service/internal/orchestrator"
	"calculator-service/internal/types"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func setupTest() {
	orchestrator.ResetState()
}

func TestHandleCalculate(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		wantStatusCode int
		wantError      bool
	}{
		{
			name:           "простое выражение",
			requestBody:    `{"expression": "2+2"}`,
			wantStatusCode: http.StatusOK,
			wantError:      false,
		},
		{
			name:           "выражение с приоритетом операций",
			requestBody:    `{"expression": "2+2*2"}`,
			wantStatusCode: http.StatusOK,
			wantError:      false,
		},
		{
			name:           "выражение со скобками",
			requestBody:    `{"expression": "(2+2)*2"}`,
			wantStatusCode: http.StatusOK,
			wantError:      false,
		},
		{
			name:           "некорректное выражение",
			requestBody:    `{"expression": "2+a"}`,
			wantStatusCode: http.StatusUnprocessableEntity,
			wantError:      true,
		},
		{
			name:           "пустое выражение",
			requestBody:    `{"expression": ""}`,
			wantStatusCode: http.StatusUnprocessableEntity,
			wantError:      true,
		},
		{
			name:           "неверный формат запроса",
			requestBody:    `{"expr": "2+2"}`,
			wantStatusCode: http.StatusUnprocessableEntity,
			wantError:      true,
		},
		{
			name:           "незавершенное выражение",
			requestBody:    `{"expression": "1+"}`,
			wantStatusCode: http.StatusUnprocessableEntity,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTest()

			req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			orchestrator.HandleCalculate(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("HandleCalculate() код статуса = %v, ожидается %v", w.Code, tt.wantStatusCode)
			}

			if !tt.wantError {
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Невозможно распарсить ответ: %v", err)
				}

				if _, exists := response["id"]; !exists {
					t.Errorf("HandleCalculate() ответ не содержит поле id")
				}
			}
		})
	}
}

func TestHandleGetExpressions(t *testing.T) {
	setupTest()

	calcReq := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", strings.NewReader(`{"expression": "2+2*2"}`))
	calcReq.Header.Set("Content-Type", "application/json")
	calcW := httptest.NewRecorder()
	orchestrator.HandleCalculate(calcW, calcReq)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/expressions", nil)
	w := httptest.NewRecorder()
	orchestrator.HandleGetExpressions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HandleGetExpressions() код статуса = %v, ожидается %v", w.Code, http.StatusOK)
	}

	var response types.ExpressionResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Невозможно распарсить ответ: %v", err)
	}

	if len(response.Expressions) == 0 {
		t.Errorf("HandleGetExpressions() должен вернуть непустой список выражений")
	}
}

func TestHandleGetExpression(t *testing.T) {
	setupTest()

	calcReq := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", strings.NewReader(`{"expression": "2+2*2"}`))
	calcReq.Header.Set("Content-Type", "application/json")
	calcW := httptest.NewRecorder()
	orchestrator.HandleCalculate(calcW, calcReq)

	var calcResponse map[string]string
	err := json.Unmarshal(calcW.Body.Bytes(), &calcResponse)
	if err != nil {
		t.Fatalf("Невозможно распарсить ответ: %v", err)
	}
	exprID := calcResponse["id"]

	req := httptest.NewRequest(http.MethodGet, "/api/v1/expressions/"+exprID, nil)
	req = mux.SetURLVars(req, map[string]string{"id": exprID})
	w := httptest.NewRecorder()
	orchestrator.HandleGetExpression(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HandleGetExpression() код статуса = %v, ожидается %v", w.Code, http.StatusOK)
	}

	var expr types.Expression
	err = json.Unmarshal(w.Body.Bytes(), &expr)
	if err != nil {
		t.Fatalf("Невозможно распарсить ответ: %v", err)
	}

	if expr.ID != exprID {
		t.Errorf("HandleGetExpression() id = %v, ожидается %v", expr.ID, exprID)
	}
	if expr.Original != "2+2*2" {
		t.Errorf("HandleGetExpression() expression = %v, ожидается %v", expr.Original, "2+2*2")
	}
}

func TestHandleGetTask(t *testing.T) {
	tests := []struct {
		name             string
		expression       string
		expectedPriority int
		expectedOp       string
	}{
		{
			name:             "приоритет умножения",
			expression:       "2+2*3",
			expectedPriority: 2,
			expectedOp:       "*",
		},
		{
			name:             "приоритет деления",
			expression:       "2+6/3",
			expectedPriority: 2,
			expectedOp:       "/",
		},
		{
			name:             "приоритет сложения после умножения",
			expression:       "2*3+4",
			expectedPriority: 2,
			expectedOp:       "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupTest()

			// Добавляем выражение
			calcReq := httptest.NewRequest(http.MethodPost, "/api/v1/calculate",
				strings.NewReader(`{"expression": "`+tt.expression+`"}`))
			calcReq.Header.Set("Content-Type", "application/json")
			calcW := httptest.NewRecorder()
			orchestrator.HandleCalculate(calcW, calcReq)

			// Получаем задачу
			req := httptest.NewRequest(http.MethodGet, "/internal/task", nil)
			w := httptest.NewRecorder()
			orchestrator.HandleGetTask(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("HandleGetTask() код статуса = %v, ожидается %v", w.Code, http.StatusOK)
			}

			var task types.Task
			err := json.Unmarshal(w.Body.Bytes(), &task)
			if err != nil {
				t.Fatalf("Невозможно распарсить ответ: %v", err)
			}

			if task.Priority != tt.expectedPriority {
				t.Errorf("HandleGetTask() priority = %v, ожидается %v", task.Priority, tt.expectedPriority)
			}
			if task.Operation != tt.expectedOp {
				t.Errorf("HandleGetTask() operation = %v, ожидается %v", task.Operation, tt.expectedOp)
			}
		})
	}
}

func TestHandleSubmitTaskResult(t *testing.T) {
	setupTest()

	calcReq := httptest.NewRequest(http.MethodPost, "/api/v1/calculate",
		strings.NewReader(`{"expression": "2*3"}`))
	calcReq.Header.Set("Content-Type", "application/json")
	calcW := httptest.NewRecorder()
	orchestrator.HandleCalculate(calcW, calcReq)

	taskReq := httptest.NewRequest(http.MethodGet, "/internal/task", nil)
	taskW := httptest.NewRecorder()
	orchestrator.HandleGetTask(taskW, taskReq)

	var task types.Task
	err := json.Unmarshal(taskW.Body.Bytes(), &task)
	if err != nil {
		t.Fatalf("Невозможно распарсить ответ: %v", err)
	}

	taskResult := types.TaskResult{
		ID:     task.ID,
		Result: 6, // 2*3 = 6
	}
	resultBody, _ := json.Marshal(taskResult)

	resultReq := httptest.NewRequest(http.MethodPost, "/internal/task", bytes.NewReader(resultBody))
	resultReq.Header.Set("Content-Type", "application/json")
	resultW := httptest.NewRecorder()
	orchestrator.HandleSubmitTaskResult(resultW, resultReq)

	if resultW.Code != http.StatusOK {
		t.Errorf("HandleSubmitTaskResult() код статуса = %v, ожидается %v", resultW.Code, http.StatusOK)
	}

	var calcResponse map[string]string
	err = json.Unmarshal(calcW.Body.Bytes(), &calcResponse)
	if err != nil {
		t.Fatalf("Невозможно распарсить ответ: %v", err)
	}
	exprID := calcResponse["id"]

	exprReq := httptest.NewRequest(http.MethodGet, "/api/v1/expressions/"+exprID, nil)
	exprReq = mux.SetURLVars(exprReq, map[string]string{"id": exprID})
	exprW := httptest.NewRecorder()
	orchestrator.HandleGetExpression(exprW, exprReq)

	var expr types.Expression
	err = json.Unmarshal(exprW.Body.Bytes(), &expr)
	if err != nil {
		t.Fatalf("Невозможно распарсить ответ: %v", err)
	}

	if expr.Status != "COMPLETED" {
		t.Errorf("После отправки результата статус должен быть COMPLETED, получено %v", expr.Status)
	}
	if expr.Result != 6 {
		t.Errorf("Результат должен быть 6, получено %v", expr.Result)
	}
}

func TestOrderOfOperations(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		expected   float64
	}{
		{
			name:       "сложение и умножение",
			expression: "2+3*4",
			expected:   14, // 2+(3*4)
		},
		{
			name:       "умножение и деление",
			expression: "6*4/8",
			expected:   3, // (6*4)/8
		},
		{
			name:       "сложение, вычитание и умножение",
			expression: "5-2*3+1",
			expected:   0, // 5-(2*3)+1
		},
		{
			name:       "выражение со скобками",
			expression: "(2+3)*4",
			expected:   20, // (2+3)*4
		},
		{
			name:       "сложное выражение",
			expression: "2+3*4-6/2",
			expected:   11, // 2+(3*4)-(6/2)
		},
	}

	calc := calculator.NewCalculator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calc.Calculate(tt.expression)
			if err != nil {
				t.Fatalf("Ошибка вычисления: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Calculate(%s) = %v, ожидается %v", tt.expression, result, tt.expected)
			}
		})
	}
}
