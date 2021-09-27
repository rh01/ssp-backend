package otc

import (
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
)

type Features struct {
	UOS bool `json:"uos"`
	RDS bool `json:"rds"`
}

func GetFeatures() Features {
	cfg := config.Config()
	uosEnabled := cfg.GetString("uos_enabled")
	rdsEnabled := cfg.GetString("rds_enabled")

	return Features{
		UOS: uosEnabled == "true",
		RDS: rdsEnabled == "true",
	}
}
