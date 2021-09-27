package otc

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/auth/token"
	"github.com/gophercloud/gophercloud/openstack"
	"time"
)

const (
	genericOTCAPIError = "Error when calling the OTC API. Please create a ticket"
	wrongAPIUsageError = "Invalid API request: Argument doesn't match definition. Please create a ticket."
)

func RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/otc/ecs", listECSHandler)
	r.POST("/otc/stopecs", stopECSHandler)
	r.POST("/otc/startecs", startECSHandler)
	r.POST("/otc/rebootecs", rebootECSHandler)
	r.GET("/otc/flavors", listFlavorsHandler)
	r.GET("/otc/images", listImagesHandler)
	r.GET("/otc/rds/versions", listRDSVersionsHandler)
	r.GET("/otc/rds/flavors", listRDSFlavorsHandler)
	r.GET("/otc/rds/instances", listRDSInstancesHandler)
}

func getProvider(to *token.TokenOptions) (*gophercloud.ProviderClient, error) {
	opts, err := TokenOptionsFromEnv(to)
	if err != nil {
		return nil, err
	}

	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

func getComputeClient(domain string) (*gophercloud.ServiceClient, error) {
	to := token.TokenOptions{
		TenantName: "eu-ch_managed",
		DomainName: domain,
	}
	provider, err := getProvider(&to)
	if err != nil {
		fmt.Println("Error while authenticating.", err.Error())
		return nil, errors.New(genericOTCAPIError)
	}

	client, err := openstack.NewComputeV2(provider, gophercloud.EndpointOpts{
		Region: "eu-ch",
	})

	if err != nil {
		fmt.Println("Error getting client.", err.Error())
		return nil, errors.New(genericOTCAPIError)
	}

	return client, nil
}

func getRDSV1Client(provider *gophercloud.ProviderClient) (*gophercloud.ServiceClient, error) {
	client, err := openstack.NewRDSV1(provider, gophercloud.EndpointOpts{})
	if err != nil {
		fmt.Println("Error getting client.", err.Error())
		return nil, errors.New(genericOTCAPIError)
	}

	return client, nil
}

func getRDSClient(domain string) (*gophercloud.ServiceClient, error) {
	to := token.TokenOptions{
		TenantName: "eu-ch_rds",
		DomainName: domain,
	}
	provider, err := getProvider(&to)
	if err != nil {
		fmt.Println("Error while authenticating.", err.Error())
		return nil, errors.New(genericOTCAPIError)
	}

	client, err := openstack.NewRDSV3(provider, gophercloud.EndpointOpts{})
	if err != nil {
		fmt.Println("Error getting client.", err.Error())
		return nil, errors.New(genericOTCAPIError)
	}

	return client, nil
}

func getImageClient() (*gophercloud.ServiceClient, error) {
	provider, err := getProvider(nil)
	if err != nil {
		fmt.Println("Error while authenticating.", err.Error())
		return nil, errors.New(genericOTCAPIError)
	}

	client, err := openstack.NewImageServiceV2(provider, gophercloud.EndpointOpts{
		Region: "eu-ch",
	})

	if err != nil {
		fmt.Println("Error getting client.", err.Error())
		return nil, errors.New(genericOTCAPIError)
	}

	return client, nil
}

func getBlockStorageClient() (*gophercloud.ServiceClient, error) {
	provider, err := getProvider(nil)
	if err != nil {
		fmt.Println("Error while authenticating.", err.Error())
		return nil, errors.New(genericOTCAPIError)
	}

	client, err := openstack.NewBlockStorageV3(provider, gophercloud.EndpointOpts{
		Region: "eu-ch",
	})

	if err != nil {
		fmt.Println("Error getting client.", err.Error())
		return nil, errors.New(genericOTCAPIError)
	}

	return client, nil
}

// https://upgear.io/blog/simple-golang-retry-function/
func retry(attempts int, sleep time.Duration, fn func() error) error {
	if err := fn(); err != nil {
		if s, ok := err.(Stop); ok {
			// Return the original error for later checking
			return s.error
		}

		if attempts--; attempts > 0 {
			time.Sleep(sleep)
			return retry(attempts, 2*sleep, fn)
		}
		return err
	}
	return nil
}

type Stop struct {
	error
}
