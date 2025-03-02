document.addEventListener('DOMContentLoaded', () => {
    const expressionInput = document.getElementById('expression');
    const calculateButton = document.getElementById('calculate');
    const resultDiv = document.getElementById('result');
    const historyList = document.getElementById('history-list');

    loadHistory();

    calculateButton.addEventListener('click', () => {
        calculateExpression();
    });

    expressionInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            calculateExpression();
        }
    });

    async function calculateExpression() {
        const expression = expressionInput.value.trim();
        if (!expression) {
            showError('Пожалуйста, введите выражение');
            return;
        }
        
        try {
            resultDiv.innerHTML = '<div class="processing">Вычисление...</div>';
            
            const response = await fetch('/api/v1/calculate', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ expression })
            });
            
            if (!response.ok) {
                const errorData = await response.json();
                throw new Error(errorData.error || `Ошибка: ${response.status} ${response.statusText}`);
            }
            
            const data = await response.json();
            const expressionId = data.id;

            checkExpressionStatus(expressionId);
        } catch (error) {
            showError(error.message);
        }
    }

    async function checkExpressionStatus(id) {
        try {
            const response = await fetch(`/api/v1/expressions/${id}`);
            
            if (!response.ok) {
                throw new Error(`Ошибка при проверке статуса: ${response.status}`);
            }
            
            const data = await response.json();
            
            if (data.status === 'COMPLETED') {
                resultDiv.innerHTML = `<div class="success">Результат: ${data.result}</div>`;
                loadHistory();
            } else if (data.status === 'ERROR') {
                showError('Ошибка при вычислении');
            } else {
                resultDiv.innerHTML = '<div class="processing">Выполняется вычисление...</div>';
                setTimeout(() => checkExpressionStatus(id), 1000);
            }
        } catch (error) {
            showError('Ошибка при получении результата');
        }
    }

    async function loadHistory() {
        try {
            const response = await fetch('/api/v1/expressions');
            
            if (!response.ok) {
                throw new Error(`Ошибка при загрузке истории: ${response.status}`);
            }
            
            const data = await response.json();
            
            historyList.innerHTML = '';
            
            if (data.expressions && data.expressions.length > 0) {
                const sortedExpressions = [...data.expressions].reverse();
                
                sortedExpressions.forEach(expr => {
                    const li = document.createElement('li');
                    li.className = `history-item ${expr.status.toLowerCase()}`;
                    
                    const expressionText = document.createElement('span');
                    expressionText.className = 'expression';
                    expressionText.textContent = expr.expression;
                    
                    const statusText = document.createElement('span');
                    statusText.className = 'status';

                    let statusRu = 'В обработке';
                    if (expr.status === 'COMPLETED') statusRu = 'Готово';
                    if (expr.status === 'ERROR') statusRu = 'Ошибка';
                    
                    statusText.textContent = statusRu;
                    
                    const resultText = document.createElement('span');
                    resultText.className = 'result';
                    resultText.textContent = expr.status === 'COMPLETED' ? `= ${expr.result}` : '';
                    
                    li.appendChild(expressionText);
                    li.appendChild(statusText);
                    li.appendChild(resultText);

                    li.addEventListener('click', () => {
                        expressionInput.value = expr.expression;
                        resultDiv.scrollIntoView({ behavior: 'smooth' });
                    });
                    
                    historyList.appendChild(li);
                });
            } else {
                historyList.innerHTML = '<li class="history-item">История вычислений пуста</li>';
            }
        } catch (error) {
            historyList.innerHTML = '<li class="history-item error">Ошибка при загрузке истории</li>';
            console.error(error);
        }
    }

    function showError(message) {
        resultDiv.innerHTML = `<div class="error">${message}</div>`;
    }
}); 