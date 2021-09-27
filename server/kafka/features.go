package kafka

type Features struct {
	Enabled bool `json:"enabled"`
}

func GetFeatures() Features {
	kafkaConfig := getKafkaConfig()

	return Features{
		Enabled: kafkaConfig.BackendUrl != "",
	}
}
