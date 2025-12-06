package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// Описание одного нутриента, приходящего из Django
type CalcNutrient struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	DailyDoseMin float64 `json:"daily_dose_min"`
	DailyDoseMax float64 `json:"daily_dose_max"`
}

// Запрос на расчёт от Django
type CalcRequest struct {
	RequestID     int64              `json:"request_id"`
	BodyMass      float64            `json:"body_mass"`
	DishMass      float64            `json:"dish_mass"`
	DishNutrients map[string]float64 `json:"dish_nutrients"`
	Nutrients     []CalcNutrient     `json:"nutrients"`
}

// Один элемент результата по нутриенту
type CalcResultItem struct {
	NutrientID          int64   `json:"nutrient_id"`
	QuantityInDish      float64 `json:"quantity_in_dish"`
	DailyDosePercentage float64 `json:"daily_dose_percentage"`
}

// Полезная нагрузка, отправляемая обратно в Django
type CalcResultPayload struct {
	Results []CalcResultItem `json:"results"`
}

// Фактический расчёт и отправка результата в Django
func processCalculation(req CalcRequest) {
	// Имитация долгих вычислений
	time.Sleep(5 * time.Second)

	results := make([]CalcResultItem, 0, len(req.Nutrients))

	for _, n := range req.Nutrients {
		valuePerUnit, ok := req.DishNutrients[n.Name]
		if !ok {
			// В блюде нет такого ключа – пропускаем
			continue
		}

		// Формула из Django:
		// quantity_in_dish = dish.nutrients[name] * (dish_mass / 1000)
		quantity := valuePerUnit * (req.DishMass / 1000.0)

		// daily_dose_percentage = (quantity_in_dish / (body_mass * (daily_max + daily_min)/2)) * 100
		avgDose := (n.DailyDoseMin + n.DailyDoseMax) / 2.0
		denom := req.BodyMass * avgDose

		var percent float64
		if denom > 0 {
			percent = quantity / denom * 100.0
		} else {
			percent = 0
		}

		results = append(results, CalcResultItem{
			NutrientID:          n.ID,
			QuantityInDish:      quantity,
			DailyDosePercentage: percent,
		})
	}

	payload := CalcResultPayload{Results: results}

	djangoBaseURL := os.Getenv("DJANGO_BASE_URL")
	if djangoBaseURL == "" {
		djangoBaseURL = "http://localhost:8000"
	}

	asyncToken := os.Getenv("ASYNC_SERVICE_TOKEN")
	if asyncToken == "" {
		asyncToken = "super-secret-async-token"
	}

	url := fmt.Sprintf("%s/api/v1/dish_compositions/%d/calc_result", djangoBaseURL, req.RequestID)

	body, err := json.Marshal(payload)
	if err != nil {
		log.Println("failed to marshal payload:", err)
		return
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		log.Println("failed to create request:", err)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-ASYNC-TOKEN", asyncToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Println("failed to call django:", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("Callback sent to %s, status: %d\n", url, resp.StatusCode)
}

// Запуск HTTP-сервера Go
func StartServer() {
	router := gin.Default()

	// Один метод для приёма задач на расчёт
	router.POST("/calc_nutrients", func(ctx *gin.Context) {

		var req CalcRequest
		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}

		// Запускаем расчёт в отдельной горутине
		go processCalculation(req)

		ctx.JSON(http.StatusOK, gin.H{"message": "calculation started"})
	})

	// Слушаем, например, 8081 порт
	router.Run(":8081")
}
