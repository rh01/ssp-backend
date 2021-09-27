package openshift

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"fmt"

	"github.com/Jeffail/gabs/v2"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/common"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
	"github.com/gin-gonic/gin"
)

const (
	getQuotasApiError = "Error getting quotas from ose-api: %v"
	jsonDecodingError = "Error decoding json from ose api: %v"
)

func getQuotasHandler(c *gin.Context) {
	username := common.GetUserName(c)

	params := c.Request.URL.Query()
	clusterId := params.Get("clusterid")
	project := params.Get("project")

	if err := validateAdminAccess(clusterId, username, project); err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		return
	}

	quotas, err := getQuotas(clusterId, project)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
	}

	c.JSON(http.StatusOK, quotas.String())
}

func getQuotas(clusterId, project string) (*gabs.Container, error) {
	resp, err := getOseHTTPClient("GET", clusterId, "api/v1/namespaces/"+project+"/resourcequotas", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	json, err := gabs.ParseJSONBuffer(resp.Body)
	if err != nil {
		log.Printf(jsonDecodingError, err)
		return nil, errors.New(genericAPIError)
	}

	return json.S("items").Index(0), nil
}

func editQuotasHandler(c *gin.Context) {
	username := common.GetUserName(c)

	var data common.EditQuotasCommand
	if c.BindJSON(&data) == nil {
		if err := validateEditQuotas(data.ClusterId, username, data.Project, data.CPU, data.Memory); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
			return
		}

		if err := updateQuotas(data.ClusterId, username, data.Project, data.CPU, data.Memory); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		} else {
			c.JSON(http.StatusOK, common.ApiResponse{
				Message: fmt.Sprintf("The new quotas have been saved: Cluster %v, Project %v, CPU: %v, Memory: %v",
					data.ClusterId, data.Project, data.CPU, data.Memory),
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
	}
}

func validateEditQuotas(clusterId, username, project string, cpu int, memory int) error {
	cfg := config.Config()
	maxCPU := cfg.GetInt("max_quota_cpu")
	maxMemory := cfg.GetInt("max_quota_memory")

	if maxCPU == 0 || maxMemory == 0 {
		log.Println("WARNING: Env variables 'MAX_QUOTA_MEMORY' and 'MAX_QUOTA_CPU' must be specified and valid integers")
		return errors.New(common.ConfigNotSetError)
	}

	// Validate user input
	if clusterId == "" {
		return errors.New("Cluster must be provided")
	}

	if project == "" {
		return errors.New("Project must be provided")
	}

	if cpu > maxCPU {
		return fmt.Errorf("The maximal value for CPU cores: %v", maxCPU)
	}

	if memory > maxMemory {
		return fmt.Errorf("The maximal value for memory: %v", maxMemory)
	}

	// Validate permissions
	resp := checkAdminPermissions(clusterId, username, project)
	return resp
}

func updateQuotas(clusterId, username, project string, cpu int, memory int) error {
	quotas, err := getQuotas(clusterId, project)
	if err != nil {
		return err
	}
	quotas.SetP(cpu, "spec.hard.cpu")
	quotas.SetP(fmt.Sprintf("%vGi", memory), "spec.hard.memory")

	resp, err := getOseHTTPClient("PUT",
		clusterId,
		"api/v1/namespaces/"+project+"/resourcequotas/"+quotas.Path("metadata.name").Data().(string),
		bytes.NewReader(quotas.Bytes()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errMsg, _ := ioutil.ReadAll(resp.Body)
		log.Println("Error updating resourceQuota:", resp.StatusCode, string(errMsg))
		return errors.New(genericAPIError)
	}
	log.Printf("User %v changed quotas for the project %v on cluster %v. CPU: %v Mem: %v", username, clusterId, project, cpu, memory)
	return nil
}
