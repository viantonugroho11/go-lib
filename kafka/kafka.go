// Package kafka provides a generic Kafka consumer with EventHandler[E].
//
// Usage:
//
//	c, err := kafka.NewConsumer[MyEvent](brokers, groupID, topic, myHandler, kafka.WithHeaderKeys("correlation_id"))
//	c.Start(ctx)
//	defer c.Close()
package kafka
