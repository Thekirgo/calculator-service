package api

import (
	"calculator-service/internal/calculator"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type CalculatorHandler struct{}

func NewCalculatorHandler() *CalculatorHandler {
	return &CalculatorHandler{}
}

type CalculateRequest struct {
	Expression string `json:"expression"`
}

func (h *CalculatorHandler) Calculate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		SendErrorResponse(w, http.StatusInternalServerError, "Internal server error")

		return
	}

	if len(body) == 0 {
		SendErrorResponse(w, http.StatusInternalServerError, "Internal server error")

		return
	}

	var req CalculateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		SendErrorResponse(w, http.StatusUnprocessableEntity, "Expression is not valid")

		return
	}

	if strings.TrimSpace(req.Expression) == "" {
		SendErrorResponse(w, http.StatusUnprocessableEntity, "Expression is not valid")

		return
	}

	result, err := calculator.Calc(req.Expression)
	if err != nil {
		if strings.Contains(err.Error(), "invalid") ||
			strings.Contains(err.Error(), "division by zero") ||
			strings.Contains(err.Error(), "mismatched parentheses") {
			SendErrorResponse(w, http.StatusUnprocessableEntity, "Expression is not valid")

			return
		}

		SendErrorResponse(w, http.StatusInternalServerError, "Internal server error")

		return
	}

	SendSuccessResponse(w, result)
}
