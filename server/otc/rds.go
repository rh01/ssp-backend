package otc

import (
	"fmt"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/common"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/ldap"
	"github.com/gin-gonic/gin"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/rds/v1/tags"
	"github.com/gophercloud/gophercloud/openstack/rds/v3/datastores"
	"github.com/gophercloud/gophercloud/openstack/rds/v3/flavors"
	"github.com/gophercloud/gophercloud/openstack/rds/v3/instances"
	"log"
	"net/http"
	"strings"
	"time"
)

func listRDSFlavorsHandler(c *gin.Context) {
	version := c.Request.URL.Query().Get("version_name")
	if version == "" {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: "Wrong API usage. Missing parameter version_name"})
		return
	}
	stage := c.Request.URL.Query().Get("stage")
	if stage == "" {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: "Wrong API usage. Missing parameter stage"})
		return
	}
	if stage != "p" && stage != "t" {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: fmt.Sprintf("Wrong API usage. Parameter stage is: %v. Should be p or t", stage)})
		return
	}
	tenant := fmt.Sprintf("SBB_RZ_%v_001", strings.ToUpper(stage))
	client, err := getRDSClient(tenant)
	if err != nil {
		log.Println("Error getting rds client.", err.Error())
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericOTCAPIError})
		return
	}

	dbFlavorsOpts := flavors.DbFlavorsOpts{
		Versionname: version,
	}

	allPages, err := flavors.List(client, dbFlavorsOpts, "postgresql").AllPages()
	if err != nil {
		log.Println("Error while listing flavors.", err.Error())
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: "There was a problem getting the available database flavors"})
		return
	}

	flavors, err := flavors.ExtractDbFlavors(allPages)
	if err != nil {
		log.Println("Error while extracting flavors.", err.Error())
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: "There was a problem getting the available database flavors"})
		return
	}

	c.JSON(http.StatusOK, flavors)
	return
}

func listRDSVersionsHandler(c *gin.Context) {
	stage := c.Request.URL.Query().Get("stage")
	if stage == "" {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: "Wrong API usage. Missing parameter stage"})
		return
	}
	if stage != "p" && stage != "t" {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: fmt.Sprintf("Wrong API usage. Parameter stage is: %v. Should be p or t", stage)})
		return
	}
	tenant := fmt.Sprintf("SBB_RZ_%v_001", strings.ToUpper(stage))
	client, err := getRDSClient(tenant)
	if err != nil {
		log.Println("Error getting rds client.", err.Error())
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericOTCAPIError})
		return
	}

	allPages, err := datastores.List(client, "postgresql").AllPages()
	if err != nil {
		log.Println("Error while listing datastores.", err.Error())
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: "There was a problem getting the available database versions"})
		return
	}

	datastores, err := datastores.ExtractDataStores(allPages)
	if err != nil {
		log.Println("Error while extracting datastores.", err.Error())
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: "There was a problem getting the available database versions"})
		return
	}

	cfg := config.Config()
	versionWhitelist := cfg.GetStringSlice("rds.version_whitelist")

	versions := make([]string, 0)

	for _, d := range datastores.DataStores {
		if len(versionWhitelist) == 0 || common.ContainsStringI(versionWhitelist, d.Name) {
			versions = append(versions, d.Name)
		}
	}

	c.JSON(http.StatusOK, versions)
	return
}

func listRDSInstancesHandler(c *gin.Context) {
	username := common.GetUserName(c)

	var response []rdsInstance
	tenants := []string{
		"SBB_RZ_T_001",
		"SBB_RZ_P_001",
	}
	for _, tenant := range tenants {
		client, err := getRDSClient(tenant)
		if err != nil {
			log.Println("Error getting rds client.", err.Error())
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericOTCAPIError})
			return
		}

		instances, err := getRDSInstancesByUsername(client, username)
		if err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericOTCAPIError})
			return
		}
		response = append(response, instances...)
	}
	c.JSON(http.StatusOK, response)
	return
}

type rdsInstance struct {
	instances.RdsInstanceResponse
	Tags map[string]string
}

func getRDSInstancesByUsername(client *gophercloud.ServiceClient, username string) ([]rdsInstance, error) {
	// Use make because of the following behaviour:
	// https://github.com/gin-gonic/gin/issues/125
	filteredInstances := make([]rdsInstance, 0)
	l, err := ldap.New()
	if err != nil {
		return nil, err
	}
	defer l.Close()

	groups, err := l.GetGroupsOfUser(username)
	if err != nil {
		return nil, err
	}

	instances, err := getRDSInstances(client)
	if err != nil {
		log.Println("Error getting rds client.", err.Error())
		return nil, err
	}

	clientV1, err := getRDSV1Client(client.ProviderClient)
	if err != nil {
		log.Println("Error getting rdsV1 client.", err.Error())
		return nil, err
	}

	for _, instance := range instances {
		if instance.Type == "slave" {
			continue
		}
		log.Printf("%v", instance.Nodes[0].Id)
		id, err := getMasterNodeID(instance.Nodes)
		if err != nil {
			log.Printf("Error while getting the ID for: %v", instance.Id)
			continue
		}
		t, err := getRDSTags(clientV1, id)
		if err != nil {
			continue
		}
		if t["rds_group"] == "" {
			continue
		}
		if !common.ContainsStringI(groups, t["rds_group"]) {
			continue
		}
		filteredInstances = append(filteredInstances, rdsInstance{instance, t})
		log.Printf("ALLOWED %v %v", username, instance.Id)
	}
	return filteredInstances, nil
}

func getMasterNodeID(nodes []instances.Nodes) (string, error) {
	for _, n := range nodes {
		if n.Role == "master" {
			return n.Id, nil
		}
	}
	return "", fmt.Errorf("Error getting master node id")
}

func getRDSInstances(client *gophercloud.ServiceClient) ([]instances.RdsInstanceResponse, error) {

	allPages, err := instances.List(client, nil).AllPages()
	if err != nil {
		return nil, err
	}

	instances, err := instances.ExtractRdsInstances(allPages)
	if err != nil {
		return nil, err
	}

	return instances.Instances, nil
}

func getRDSTags(client *gophercloud.ServiceClient, id string) (map[string]string, error) {
	var t map[string]string
	err := retry(5, 5*time.Second, func() error {
		var err error
		t, err = tags.GetTags(client, id).Extract()
		if err != nil {
			log.Println("Retrying...")
		}
		return err
	})
	if err != nil {
		log.Printf("Error while listing tags for instance: %v. %v", id, err)
		return nil, err
	}
	return t, nil
}
