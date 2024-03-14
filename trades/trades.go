package trades

import (
	"strings"
	"time"
)

type FeeDetails struct {
	Asset string  `bson:"asset" json:"asset"`
	Fee   float64 `bson:"fee" json:"fee"`
}

type History struct {
	Type       string       `bson:"type" json:"type"`
	Quantity   float64      `bson:"quantity" json:"quantity"`
	Price      float64      `bson:"price" json:"price"`
	Fee        float64      `bson:"fee" json:"fee"`
	FeeAsset   string       `bson:"feeAsset" json:"feeAsset"`
	FeeDetails []FeeDetails `bson:"feeDetails" json:"feeDetails"`
	OrderId    int64        `bson:"orderId" json:"orderId"`
	Status     string       `bson:"status" json:"status"`
	CreatedAt  time.Time    `json:"createdAt"`
	UpdatedAt  time.Time    `json:"updatedAt"`
}

func GetQuantities(history []History) (float64, float64) {
	var buyTotal float64
	var sellTotal float64

	for _, historyData := range history {
		if strings.ToLower(historyData.Type) == "buy" {
			buyTotal += historyData.Quantity
		} else {
			sellTotal += historyData.Quantity
		}
	}

	return buyTotal, sellTotal
}
