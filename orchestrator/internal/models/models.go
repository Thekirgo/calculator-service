package models

type Task struct {
	ID            string  `json:"id"`
	Arg1          float64 `json:"arg1"`
	Arg2          float64 `json:"arg2"`
	Operation     string  `json:"operation"`
	OperationTime int     `json:"operation_time"`
}

type TaskResult struct {
	ID     string  `json:"id"`
	Result float64 `json:"result"`
}

type Expression struct {
	ID     string  `json:"id"`
	Text   string  `json:"expression"`
	Status string  `json:"status"`
	Result float64 `json:"result,omitempty"`
}
