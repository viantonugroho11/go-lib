package handlers

import (
	"context"
	"log"

	"kafka"
)

// OrderEvent payload untuk topic order (contoh domain event).
type OrderEvent struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

// RepaymentEvent payload untuk topic repayment (contoh domain event).
type RepaymentEvent struct {
	ResourceID string `json:"resource_id"`
	Status     string `json:"status"`
}

// OrderHandler mengimplementasi kafka.EventHandler[OrderEvent]; headers tidak dipakai (bisa diabaiikan).
type OrderHandler struct{}

func (OrderHandler) Name() string { return "OrderConsumer" }

func (h OrderHandler) Handle(ctx context.Context, evt OrderEvent, _ ...kafka.Header) kafka.Progress {
	if evt.ID == "" {
		return kafka.Progress{
			Status: kafka.ProgressDrop,
			Result: "order id kosong",
		}
	}
	log.Printf("[%s] id=%s amount=%d", h.Name(), evt.ID, evt.Amount)
	return kafka.Progress{Status: kafka.ProgressSuccess, Result: "ok"}
}

// RepaymentHandler mengimplementasi kafka.EventHandler[RepaymentEvent]; memakai headers.
type RepaymentHandler struct{}

func (RepaymentHandler) Name() string { return "RepaymentConsumer" }

func (h RepaymentHandler) Handle(ctx context.Context, evt RepaymentEvent, headers ...kafka.Header) kafka.Progress {
	if evt.ResourceID == "" {
		return kafka.Progress{
			Status: kafka.ProgressDrop,
			Result: "resource_id kosong",
		}
	}
	correlationID := kafka.GetHeaderString(headers, "correlation_id")
	log.Printf("[%s] resource_id=%s status=%s correlation_id=%s", h.Name(), evt.ResourceID, evt.Status, correlationID)
	return kafka.Progress{Status: kafka.ProgressSuccess, Result: "ok"}
}

// NewOrderConsumer returns a Consumer for OrderEvent with OrderHandler.
func NewOrderConsumer(brokers []string, groupID, topic string, opts ...kafka.ConsumerOption) (kafka.Consumer, error) {
	return kafka.NewConsumer[OrderEvent](brokers, groupID, topic, OrderHandler{}, opts...)
}

// NewRepaymentConsumer returns a Consumer for RepaymentEvent with RepaymentHandler.
func NewRepaymentConsumer(brokers []string, groupID, topic string, opts ...kafka.ConsumerOption) (kafka.Consumer, error) {
	return kafka.NewConsumer[RepaymentEvent](brokers, groupID, topic, RepaymentHandler{}, opts...)
}

