package otc

import (
	"fmt"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/common"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/ldap"
	"github.com/gin-gonic/gin"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v1/volumetypes"
	"github.com/gophercloud/gophercloud/openstack/blockstorage/v3/volumes"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/keypairs"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/startstop"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	log "github.com/sirupsen/logrus"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func listECSHandler(c *gin.Context) {
	username := common.GetUserName(c)

	log.Printf("%v lists ECS instances @ OTC.", username)

	params := c.Request.URL.Query()
	showall, err := strconv.ParseBool(params.Get("showall"))
	if err != nil {
		log.Printf("Error parsing showall: %v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericOTCAPIError})
		return
	}
	allServers, err := getAllServers(username)
	if err != nil {
		log.Printf("Error getting the servers: %v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericOTCAPIError})
		return
	}

	filteredServers, err := filterServersByUsername(username, allServers, showall)
	if err != nil {
		log.Printf("Error filtering ECS servers: %v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericOTCAPIError})
		return
	}

	if filteredServers == nil {
		filteredServers = []servers.Server{}
	}

	c.JSON(http.StatusOK, ECServerListResponse{Servers: filteredServers})
}

func listFlavorsHandler(c *gin.Context) {
	log.Println("Querying flavors @ OTC.")
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
	client, err := getComputeClient(tenant)

	if err != nil {
		fmt.Println("Error getting compute client.", err.Error())
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericOTCAPIError})
		return
	}

	allFlavors, err := getFlavors(client)

	if err != nil {
		log.Println("Error getting flavors.", err.Error())
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericOTCAPIError})
		return
	}

	c.JSON(http.StatusOK, allFlavors)
	return
}

func listImagesHandler(c *gin.Context) {
	type labelValue struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}
	images := []labelValue{}
	err := config.Config().UnmarshalKey("uos.images", &images)
	if err != nil {
		log.Printf("Error getting images: %v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: common.ConfigNotSetError})
		return
	}
	if len(images) == 0 {
		log.Printf("Error: no images found in config (uos.images)")
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: common.ConfigNotSetError})
		return
	}
	for _, i := range images {
		if i.Label == "" || i.Value == "" {
			log.Printf("Error: missing label or value in image: %+v", i)
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: common.ConfigNotSetError})
			return
		}
	}

	c.JSON(http.StatusOK, images)
	return
}

func getComputeClients() (map[string]*gophercloud.ServiceClient, error) {
	tenants := []string{
		"SBB_RZ_T_001",
		"SBB_RZ_P_001",
	}
	clients := make(map[string]*gophercloud.ServiceClient)
	var err error
	for _, tenant := range tenants {
		clients[tenant], err = getComputeClient(tenant)
		if err != nil {
			return clients, err
		}
	}
	return clients, nil
}

func getTenantName(servername string) string {
	pattern := regexp.MustCompile(`(.)\d{2}\.sbb\.ch`)
	matches := pattern.FindStringSubmatch(servername)
	if len(matches) == 2 {
		stage := matches[1]
		return fmt.Sprintf("SBB_RZ_%v_001", strings.ToUpper(stage))
	}
	return ""
}

func stopECSHandler(c *gin.Context) {
	log.Println("Stopping ECS @ OTC.")
	username := common.GetUserName(c)

	clients, err := getComputeClients()
	if err != nil {
		log.Printf("Error getting compute client: %v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericOTCAPIError})
		return
	}

	var data ECServerListResponse
	err = c.BindJSON(&data)
	if err != nil {
		log.Println("Binding request to Go struct failed.", err.Error())
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
		return
	}
	if err := validatePermissions(data.Servers, username); err != nil {
		c.JSON(http.StatusForbidden, common.ApiResponse{Message: err.Error()})
		return
	}

	for _, server := range data.Servers {
		tenant := getTenantName(server.Name)
		stopResult := startstop.Stop(clients[tenant], server.ID)

		if stopResult.Err != nil {
			log.Println("Error while stopping server.", err.Error())
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: "At least one server couldn't be stopped."})
			return
		}
	}

	c.JSON(http.StatusOK, common.ApiResponse{Message: "Server stop initiated."})
	return
}

func startECSHandler(c *gin.Context) {
	log.Println("Starting ECS @ OTC.")
	username := common.GetUserName(c)

	clients, err := getComputeClients()
	if err != nil {
		log.Printf("Error getting compute clients: %v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericOTCAPIError})
		return
	}

	var data ECServerListResponse
	err = c.BindJSON(&data)
	if err != nil {
		log.Println("Binding request to Go struct failed.", err.Error())
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
		return
	}
	if err := validatePermissions(data.Servers, username); err != nil {
		c.JSON(http.StatusForbidden, common.ApiResponse{Message: err.Error()})
		return
	}
	for _, server := range data.Servers {
		tenant := getTenantName(server.Name)
		stopResult := startstop.Start(clients[tenant], server.ID)

		if stopResult.Err != nil {
			log.Println("Error while starting server.", err.Error())
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: "At least one server couldn't be started."})
			return
		}
	}

	c.JSON(http.StatusOK, common.ApiResponse{Message: "Server start initiated."})
	return
}

func rebootECSHandler(c *gin.Context) {
	log.Println("Rebooting ECS @ OTC.")
	username := common.GetUserName(c)

	clients, err := getComputeClients()
	if err != nil {
		log.Printf("Error getting compute client: %v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericOTCAPIError})
		return
	}

	var data ECServerListResponse
	err = c.BindJSON(&data)

	if err != nil {
		log.Println("Binding request to Go struct failed.", err.Error())
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
		return
	}

	rebootOpts := servers.RebootOpts{
		Type: servers.SoftReboot,
	}

	if err := validatePermissions(data.Servers, username); err != nil {
		c.JSON(http.StatusForbidden, common.ApiResponse{Message: err.Error()})
		return
	}
	for _, server := range data.Servers {
		tenant := getTenantName(server.Name)
		rebootResult := servers.Reboot(clients[tenant], server.ID, &rebootOpts)

		if rebootResult.Err != nil {
			log.Printf("Error while rebooting server: %v", rebootResult.Err)
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: "At least one server couldn't be rebooted."})
			return
		}
	}
	c.JSON(http.StatusOK, common.ApiResponse{Message: "Reboot initiated."})
	return
}

func ValidatePermissionsByHostname(servername string, username string) error {
	if servername == "" || username == "" {
		log.WithFields(log.Fields{
			"username":   username,
			"servername": servername,
		}).Error("Empty servername or username")
		return fmt.Errorf(genericOTCAPIError)
	}
	groups, err := getGroups(username)
	if err != nil {
		return err
	}
	if common.ContainsStringI(groups, "DG_RBT_UOS_ADMINS") {
		// skip checks
		return nil
	}
	allServers, err := getAllServers(username)
	if err != nil {
		return err
	}
	var server servers.Server
	for _, s := range allServers {
		if servername == s.Name {
			server = s
			break
		}
	}
	if server.ID == "" {
		log.WithFields(log.Fields{
			"username":   username,
			"servername": servername,
		}).Error("No server found with that name")
		return fmt.Errorf(genericOTCAPIError)
	}
	group := server.Metadata["uos_group"]
	if group == "" {
		log.WithFields(log.Fields{
			"username": username,
			"server":   server.ID,
			"metadata": server.Metadata,
		}).Error("uos_group not found in metadata")
		return fmt.Errorf(genericOTCAPIError)
	}
	log.Printf("group: %v", group)
	if !common.ContainsStringI(groups, group) {
		log.WithFields(log.Fields{
			"username": username,
			"groups":   groups,
			"server":   server.ID,
			"metadata": server.Metadata,
		}).Error("uos_group not found in user groups")
		return fmt.Errorf(genericOTCAPIError)
	}
	return nil
}

func validatePermissions(untrustedServers []servers.Server, username string) error {
	groups, err := getGroups(username)
	if err != nil {
		return err
	}
	if common.ContainsStringI(groups, "DG_RBT_UOS_ADMINS") {
		// skip checks
		return nil
	}
	allServers, err := getAllServers(username)
	if err != nil {
		return err
	}
	for _, server := range untrustedServers {
		// do not trust user data, because the metadata could have been modified
		group := ""
		for _, s := range allServers {
			if server.ID == s.ID {
				group = s.Metadata["uos_group"]
				break
			}
		}
		if group == "" {
			log.WithFields(log.Fields{
				"username": username,
				"server":   server.ID,
				"metadata": server.Metadata,
			}).Error("uos_group not found in metadata")
			return fmt.Errorf(genericOTCAPIError)
		}
		if !common.ContainsStringI(groups, group) {
			log.WithFields(log.Fields{
				"username": username,
				"groups":   groups,
				"server":   server.ID,
				"metadata": server.Metadata,
			}).Error("uos_group not found in user groups")
			return fmt.Errorf(genericOTCAPIError)
		}
	}
	return nil
}

func createKeyPair(client *gophercloud.ServiceClient, publicKeyName string, publicKey string) (*keypairs.KeyPair, error) {
	log.Printf("Creating public key with name %v.", publicKeyName)

	createOpts := keypairs.CreateOpts{
		Name:      publicKeyName,
		PublicKey: publicKey,
	}

	keyPair, err := keypairs.Create(client, createOpts).Extract()

	if err != nil {
		log.Println("Error while creating key pair.", err.Error())
		return nil, err
	}

	return keyPair, nil
}

type otcTenantCache struct {
	LastRunTimestamp string
	Servers          []servers.Server
}

// Cache for all tenants
var otcCache map[string]otcTenantCache

func getAllServers(username string) ([]servers.Server, error) {
	clients, err := getComputeClients()
	if err != nil {
		return nil, err
	}
	var allServers []servers.Server
	for _, client := range clients {
		serversInTenant, err := getServers(client, username)
		if err != nil {
			return nil, err
		}
		allServers = append(allServers, serversInTenant...)
	}
	return allServers, nil
}

func getServers(client *gophercloud.ServiceClient, username string) ([]servers.Server, error) {
	if otcCache == nil {
		otcCache = make(map[string]otcTenantCache)
	}
	log.WithFields(log.Fields{
		"username": username,
	}).Debug("Getting EC Servers.")

	cacheKey := client.Endpoint
	// this seems to work even if lastRunTimestamp is empty
	opts := servers.ListOpts{
		ChangesSince: otcCache[cacheKey].LastRunTimestamp,
	}

	allPages, err := servers.List(client, opts).AllPages()
	if err != nil {
		log.WithFields(log.Fields{
			"username": username,
			"err":      err.Error(),
		}).Error("Error while listing servers")
		return nil, fmt.Errorf(genericOTCAPIError)
	}

	newServers, err := servers.ExtractServers(allPages)
	if err != nil {
		log.WithFields(log.Fields{
			"username": username,
			"err":      err.Error(),
		}).Error("Error while extracting servers")
		return nil, fmt.Errorf(genericOTCAPIError)
	}

	otcCache[cacheKey] = otcTenantCache{
		LastRunTimestamp: time.Now().Format(time.RFC3339),
		Servers:          mergeServers(otcCache[cacheKey].Servers, newServers),
	}

	return otcCache[cacheKey].Servers, nil
}

func filterServersByUsername(username string, s []servers.Server, showall bool) ([]servers.Server, error) {
	groups, err := getGroups(username)
	if err != nil {
		return nil, err
	}
	log.WithFields(log.Fields{
		"groups":   groups,
		"username": username,
	}).Debug("LDAP groups")

	if showall && common.ContainsStringI(groups, "DG_RBT_UOS_ADMINS") {
		return s, nil
	}

	var filteredServers []servers.Server
	for _, server := range s {
		if common.ContainsStringI(groups, server.Metadata["uos_group"]) {
			filteredServers = append(filteredServers, server)
		}
	}
	return filteredServers, nil
}

func getGroups(username string) ([]string, error) {
	l, err := ldap.New()
	if err != nil {
		log.WithFields(log.Fields{
			"username": username,
			"err":      err.Error(),
		}).Error("Error creating ldap object")
		return nil, fmt.Errorf(genericOTCAPIError)
	}
	defer l.Close()

	groups, err := l.GetGroupsOfUser(username)
	if err != nil {
		log.WithFields(log.Fields{
			"username": username,
			"err":      err.Error(),
		}).Error("Error getting ldap groups")
		return nil, fmt.Errorf(genericOTCAPIError)
	}
	return groups, nil
}

func mergeServers(cachedServers, newServers []servers.Server) []servers.Server {
	unique := make(map[string]servers.Server)

	for _, s := range cachedServers {
		unique[s.ID] = s
	}
	for _, s := range newServers {
		unique[s.ID] = s
	}
	final := make([]servers.Server, 0)
	for _, s := range unique {
		final = append(final, s)
	}
	return final
}

func getVolumesByServerID(client *gophercloud.ServiceClient, serverId string) ([]volumes.Volume, error) {
	var result []volumes.Volume

	opts := volumes.ListOpts{}

	allPages, err := volumes.List(client, opts).AllPages()

	if err != nil {
		log.Println("Error while listing volumes.", err.Error())
		return nil, err
	}

	allVolumes, err := volumes.ExtractVolumes(allPages)
	if err != nil {
		log.Println("Error while extracting volumes.", err.Error())
		return nil, err
	}

	for _, volume := range allVolumes {
		for _, attachment := range volume.Attachments {
			if attachment.ServerID == serverId {
				result = append(result, volume)
				continue
			}
		}
	}

	return result, err
}

func getVolumeTypes(client *gophercloud.ServiceClient) (*VolumeTypesListResponse, error) {
	log.Println("Getting volume types @ OTC.")

	result := VolumeTypesListResponse{
		VolumeTypes: []VolumeType{},
	}

	allPages, err := volumetypes.List(client).AllPages()

	if err != nil {
		log.Println("Error while listing volume types.", err.Error())
		return nil, err
	}

	allVolumeTypes, err := volumetypes.ExtractVolumeTypes(allPages)

	if err != nil {
		log.Println("Error while extracting volume types.", err.Error())
		return nil, err
	}

	for _, volumeType := range allVolumeTypes {
		result.VolumeTypes = append(result.VolumeTypes, VolumeType{Name: volumeType.Name, Id: volumeType.ID})
	}

	return &result, nil
}

func getFlavors(client *gophercloud.ServiceClient) (*FlavorListResponse, error) {
	log.Println("Getting flavors @ OTC.")

	result := FlavorListResponse{
		Flavors: []Flavor{},
	}

	opts := flavors.ListOpts{}

	allPages, err := flavors.ListDetail(client, opts).AllPages()

	if err != nil {
		log.Println("Error while listing flavors.", err.Error())
		return nil, err
	}

	allFlavors, err := flavors.ExtractFlavors(allPages)

	if err != nil {
		log.Println("Error while extracting flavors.", err.Error())
		return nil, err
	}

	for _, flavor := range allFlavors {
		result.Flavors = append(result.Flavors, Flavor{Name: flavor.Name, VCPUs: flavor.VCPUs, RAM: flavor.RAM})
	}

	return &result, nil
}
