package main

import (
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/aws"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/kafka"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/keycloak"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/ldap"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/openshift"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/otc"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/sematext"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/tower"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func main() {
	config.Init("bla")

	log.SetReportCaller(true)

	if config.Config().GetBool("debug") {
		log.SetLevel(log.DebugLevel)
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Allow cors
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AddAllowHeaders("authorization", "*")
	corsConfig.AddAllowMethods("DELETE")
	router.Use(cors.New(corsConfig))

	// Public routes
	router.GET("/features", featuresHandler)

	// Protected routes
	auth := router.Group("/api/")
	auth.Use(keycloak.Auth(keycloak.LoggedInCheck()))
	{
		// Openshift routes
		openshift.RegisterRoutes(auth)

		// AWS routes
		aws.RegisterRoutes(auth)

		// OTC routes
		otc.RegisterRoutes(auth)

		// Sematext routes
		sematext.RegisterRoutes(auth)

		// Ansible Tower
		tower.RegisterRoutes(auth)

		// Kafka routes
		kafka.RegisterRoutes(auth)

		// LDAP routes
		ldap.RegisterRoutes(auth)
	}

	log.Println("Cloud SSP is running")

	port := config.Config().GetString("port")
	if port == "" {
		port = "8000"
	}
	err := router.Run(":" + port)
	if err != nil {
		log.Println(err)
	}
}

// not in common package, because that generates an import loop
type featureToggleResponse struct {
	Openshift openshift.Features `json:"openshift"`
	OTC       otc.Features       `json:"otc"`
	Kafka     kafka.Features     `json:"kafka"`
}

func featuresHandler(c *gin.Context) {
	params := c.Request.URL.Query()
	clusterId := params.Get("clusterid")
	c.JSON(http.StatusOK, featureToggleResponse{
		Openshift: openshift.GetFeatures(clusterId),
		OTC:       otc.GetFeatures(),
		Kafka:     kafka.GetFeatures(),
	})
}
