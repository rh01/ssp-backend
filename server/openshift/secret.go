package openshift

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net/http"

	"fmt"

	"encoding/json"
	"github.com/Jeffail/gabs/v2"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/common"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
	"github.com/gin-gonic/gin"
)

type DockerConfig struct {
	Auths map[string]*Auth `json:"auths"`
}
type Auth struct {
	// byte arrays are marshalled to base64
	Auth []byte `json:"auth"`
}

func newPullSecretHandler(c *gin.Context) {
	username := common.GetUserName(c)
	cfg := config.Config()
	dockerRepository := cfg.GetString("docker_repository")
	if dockerRepository == "" {
		log.Println("Env variable 'docker_repository' must be specified")
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: common.ConfigNotSetError})
		return
	}

	var data common.NewPullSecretCommand
	if c.BindJSON(&data) != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
		return
	}
	secret := newObjectRequest("Secret", "external-registry", "v1")
	dockerConfig := DockerConfig{
		Auths: make(map[string]*Auth),
	}
	auth := Auth{
		Auth: []byte(fmt.Sprintf("%v:%v", data.Username, data.Password)),
	}
	dockerConfig.Auths[dockerRepository] = &auth
	secretData, _ := json.Marshal(dockerConfig)

	secret.Set(secretData, "data", ".dockerconfigjson")
	secret.Set("kubernetes.io/dockerconfigjson", "type")
	if err := createSecret(data.ClusterId, data.Project, secret); err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		return
	}
	if err := addPullSecretToServiceaccount(data.ClusterId, data.Project, "default"); err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		return
	}
	log.Printf("%v created a new pull secret to default serviceaccount on project %v on cluster %v", username, data.Project, data.ClusterId)
	c.JSON(http.StatusOK, common.ApiResponse{Message: "Das Pull-Secret wurde angelegt"})
}

func addPullSecretToServiceaccount(clusterId, namespace string, serviceaccount string) error {
	url := fmt.Sprintf("api/v1/namespaces/%v/serviceaccounts/%v", namespace, serviceaccount)
	patch := []common.JsonPatch{
		{
			Operation: "add",
			Path:      "/imagePullSecrets/-",
			Value: struct {
				Name string `json:"name"`
			}{
				Name: "external-registry",
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		log.Printf("Error marshalling patch: %v", err)
		return errors.New(genericAPIError)
	}

	resp, err := getOseHTTPClient("PATCH", clusterId, url, bytes.NewBuffer(patchBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Error adding pull secret to service account on cluster %v: StatusCode: %v, Nachricht: %v", clusterId, resp.StatusCode, string(bodyBytes))
		return errors.New(genericAPIError)
	}

	return nil

}

func createSecret(clusterId, namespace string, secret *gabs.Container) error {
	url := fmt.Sprintf("api/v1/namespaces/%v/secrets", namespace)

	resp, err := getOseHTTPClient("POST", clusterId, url, bytes.NewReader(secret.Bytes()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Error creating secret on cluster %v: StatusCode: %v, Nachricht: %v", clusterId, resp.StatusCode, string(bodyBytes))
		return errors.New(genericAPIError)
	}

	if resp.StatusCode == http.StatusConflict {
		return errors.New("The secret already exists")
	}

	return nil
}
