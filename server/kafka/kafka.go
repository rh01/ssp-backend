package kafka

import (
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

type KafkaConfig struct {
	BackendUrl string `json:"backend_url" mapstructure:"backend_url"`
	BillingUrl string `json:"billing_url" mapstructure:"billing_url"`
}

func getKafkaConfig() KafkaConfig {
	kafkaConfig := KafkaConfig{}
	err := config.Config().UnmarshalKey("kafka", &kafkaConfig)

	if err != nil {
		log.Println("Error unmarshalling kafka config.", err.Error())
	}

	return kafkaConfig
}

func RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/kafka/backend", getKafkaBackendHandler)
}

func getKafkaBackendHandler(c *gin.Context) {
	kafkaConfig := getKafkaConfig()
	c.JSON(http.StatusOK, kafkaConfig)
	return
}
