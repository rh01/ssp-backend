package sematext

import (
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"

	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
	"github.com/gin-gonic/gin"
	"strings"
)

const (
	wrongAPIUsageError = "Invalid api call - parameters did not match to method definition"
)

func RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/sematext/plans", getLogsenePlansHandler)
	r.GET("/sematext/discountcode", getLogseneDiscountcodeHandler)
	r.GET("/sematext/logsene", getLogseneAppsHandler)
	r.POST("/sematext/logsene", createLogseneAppHandler)
	r.POST("/sematext/logsene/:appId", updateLogseneBillingHandler)
	r.POST("/sematext/logsene/:appId/plan", updateLogsenePlanAndLimitHandler)
}

func getSematextHTTPClient(method string, urlPart string, body io.Reader) (*http.Client, *http.Request) {
	cfg := config.Config()
	token := cfg.GetString("sematext_api_token")
	baseUrl := cfg.GetString("sematext_base_url")
	if token == "" || baseUrl == "" {
		log.Fatal("Env variables 'SEMATEXT_API_TOKEN' and 'SEMATEXT_BASE_URL' must be specified")
	}

	if !strings.HasSuffix(baseUrl, "/") {
		baseUrl += "/"
	}

	client := &http.Client{}
	req, _ := http.NewRequest(method, baseUrl+urlPart, body)

	log.Debugf("Calling %v", req.URL.String())

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "apiKey "+token)

	return client, req
}
