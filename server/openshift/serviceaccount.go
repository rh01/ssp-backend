package openshift

import (
	"bytes"
	"errors"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"

	"fmt"

	"encoding/base64"
	"encoding/json"
	"github.com/Jeffail/gabs"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/common"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
	"github.com/gin-gonic/gin"
	"strings"
	"time"
)

type newJenkinsCredentialsCommand struct {
	OrganizationKey string `json:"organizationKey"`
	Secret          string `json:"secret"`
	Description     string `json:"description"`
}

func newServiceAccountHandler(c *gin.Context) {
	jenkinsUrl := config.Config().GetString("jenkins_url")
	if jenkinsUrl == "" {
		log.Fatal("Env variable 'JENKINS_URL' must be specified")
	}

	username := common.GetUserName(c)

	var data common.NewServiceAccountCommand
	if c.BindJSON(&data) != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
		return
	}

	if err := validateNewServiceAccount(data.ClusterId, username, data.Project, data.ServiceAccount); err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		return
	}

	if err := createNewServiceAccount(data.ClusterId, username, data.Project, data.ServiceAccount); err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		return
	}

	if err := authorizeServiceAccount(data.ClusterId, data.Project, data.ServiceAccount); err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		return
	}

	if len(data.OrganizationKey) > 0 {

		if err := createJenkinsCredential(data.ClusterId, data.Project, data.ServiceAccount, data.OrganizationKey); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, common.ApiResponse{
			Message: fmt.Sprintf("Service account %v has been created and stored as a Jenkins credential. You can find the credential id in Jenkins <a href='%v' target='_blank'>here</a>",
				data.ServiceAccount, jenkinsUrl+"/job/"+data.OrganizationKey+"/credentials"),
		})

	} else {
		c.JSON(http.StatusOK, common.ApiResponse{
			Message: fmt.Sprintf("The service account %v has been created", data.ServiceAccount),
		})
	}
}

func validateNewServiceAccount(clusterId, username string, project string, serviceAccountName string) error {
	if len(serviceAccountName) == 0 {
		return errors.New("You have to create a service account")
	}

	// Validate permissions
	if err := checkAdminPermissions(clusterId, username, project); err != nil {
		return err
	}

	return nil
}

func createNewServiceAccount(clusterId, username, project, serviceaccount string) error {
	p := newObjectRequest("ServiceAccount", serviceaccount, "v1")

	resp, err := getOseHTTPClient("POST", clusterId, "api/v1/namespaces/"+project+"/serviceaccounts", bytes.NewReader(p.Bytes()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return errors.New("Der Service-Account existiert bereits.")
	}

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.WithFields(log.Fields{
			"cluster":    clusterId,
			"username":   username,
			"statuscode": resp.StatusCode,
			"err":        string(bodyBytes),
		}).Error("Error creating service account")
		return errors.New(genericAPIError)
	}

	log.WithFields(log.Fields{
		"cluster":        clusterId,
		"username":       username,
		"serviceaccount": serviceaccount,
		"project":        project,
	}).Info("Serviceaccount was created")

	return nil
}

func authorizeServiceAccount(clusterId, namespace, serviceaccount string) error {
	rolebinding, err := getEditRoleBinding(clusterId, namespace)
	if err != nil {
		return err
	}
	if rolebinding == nil {
		if err := createEditRoleBinding(clusterId, namespace, serviceaccount); err != nil {
			return err
		}
		return nil
	}
	if err := addEditServiceAccountToRoleBinding(clusterId, namespace, serviceaccount, rolebinding); err != nil {
		return err
	}
	return nil
}

func addEditServiceAccountToRoleBinding(clusterId, namespace, serviceaccount string, rolebinding *gabs.Container) error {

	service_account := OpenshiftSubject{
		Kind:      "ServiceAccount",
		Name:      serviceaccount,
		Namespace: namespace,
	}
	rolebinding.ArrayAppend(service_account, "subjects")

	url := fmt.Sprintf("apis/rbac.authorization.k8s.io/v1/namespaces/%v/rolebindings/edit", namespace)
	resp, err := getOseHTTPClient("PUT", clusterId, url, bytes.NewReader(rolebinding.Bytes()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusBadRequest {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.WithFields(log.Fields{
			"cluster":        clusterId,
			"namespace":      namespace,
			"serviceaccount": serviceaccount,
			"statuscode":     resp.StatusCode,
			"err":            string(bodyBytes),
		}).Error("Error adding service account to edit rolebinding")
		return errors.New(genericAPIError)
	}

	log.WithFields(log.Fields{
		"cluster":        clusterId,
		"namespace":      namespace,
		"serviceaccount": serviceaccount,
	}).Info("Successfully added serviceaccount to edit rolebinding")

	return nil
}

func getEditRoleBinding(clusterId, namespace string) (*gabs.Container, error) {

	url := fmt.Sprintf("apis/rbac.authorization.k8s.io/v1/namespaces/%v/rolebindings/edit", namespace)
	resp, err := getOseHTTPClient("GET", clusterId, url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusBadRequest {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.WithFields(log.Fields{
			"cluster":    clusterId,
			"namespace":  namespace,
			"statuscode": resp.StatusCode,
			"err":        string(bodyBytes),
		}).Error("Error getting edit rolebinding")
		return nil, errors.New(genericAPIError)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	json, err := gabs.ParseJSONBuffer(resp.Body)
	if err != nil {
		log.Error(err.Error())
		return nil, errors.New(genericAPIError)
	}
	return json, nil
}

//FIXME: why does this work?
func createEditRoleBinding(clusterId, namespace, serviceaccount string) error {
	rolebinding := newObjectRequest("RoleBinding", "edit", "authorization.openshift.io/v1")
	rolebinding.Set("edit", "roleRef", "name")
	rolebinding.Array("userNames")
	rolebinding.ArrayAppend("system:serviceaccount:"+namespace+":"+serviceaccount, "userNames")

	url := fmt.Sprintf("apis/authorization.openshift.io/v1/namespaces/%v/rolebindings", namespace)

	resp, err := getOseHTTPClient("POST", clusterId, url, bytes.NewReader(rolebinding.Bytes()))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusBadRequest {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.WithFields(log.Fields{
			"cluster":    clusterId,
			"namespace":  namespace,
			"statuscode": resp.StatusCode,
			"err":        string(bodyBytes),
		}).Error("Error creating edit rolebinding")
		return errors.New(genericAPIError)
	}

	if resp.StatusCode == http.StatusConflict {
		return errors.New("The role binding already exists")
	}

	log.WithFields(log.Fields{
		"cluster":        clusterId,
		"namespace":      namespace,
		"serviceaccount": serviceaccount,
	}).Info("Successfully created edit rolebinding")

	return nil
}

func getServiceAccount(clusterId, namespace, serviceaccount string) (*gabs.Container, error) {
	url := fmt.Sprintf("api/v1/namespaces/%v/serviceaccounts/%v", namespace, serviceaccount)
	resp, err := getOseHTTPClient("GET", clusterId, url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.WithFields(log.Fields{
			"cluster":        clusterId,
			"namespace":      namespace,
			"serviceaccount": serviceaccount,
			"statuscode":     resp.StatusCode,
			"err":            string(bodyBytes),
		}).Error("Error getting serviceaccount")
		return nil, errors.New(genericAPIError)
	}

	json, err := gabs.ParseJSONBuffer(resp.Body)
	if err != nil {
		log.Error(err.Error())
		return nil, errors.New(genericAPIError)
	}
	return json, nil
}

func getSecret(clusterId, namespace, secret string) (*gabs.Container, error) {
	url := fmt.Sprintf("api/v1/namespaces/%v/secrets/%v", namespace, secret)
	resp, err := getOseHTTPClient("GET", clusterId, url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.Printf("Error getting secret: StatusCode: %v, Nachricht: %v", resp.StatusCode, string(bodyBytes))
		return nil, errors.New(genericAPIError)
	}

	json, err := gabs.ParseJSONBuffer(resp.Body)
	if err != nil {
		log.Println(err.Error())
		return nil, errors.New(genericAPIError)
	}
	return json, nil
}

func callWZUBackend(command newJenkinsCredentialsCommand) error {
	byteJson, err := json.Marshal(command)
	if err != nil {
		log.Println(err.Error())
		return errors.New(genericAPIError)
	}

	resp, err := getWZUBackendClient("POST", "sec/jenkins/credentials", bytes.NewReader(byteJson))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Error from WZU backend: StatusCode: %v, Message: %v", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func createJenkinsCredential(clusterId, project, serviceaccount, organizationKey string) error {
	//Sleep which ensures that the serviceaccount is created completely before we take the Secret out of it.
	time.Sleep(400 * time.Millisecond)

	saJson, err := getServiceAccount(clusterId, project, serviceaccount)
	if err != nil {
		return err
	}

	secret := saJson.S("secrets").Index(0)
	secretName := strings.Trim(secret.Path("name").String(), "\"")

	// The secret with dockercfg is the wrong one. In some rare cases this is the first secret returned
	if strings.Contains(secretName, "dockercfg") {
		secret = saJson.S("secrets").Index(1)
		secretName = strings.Trim(secret.Path("name").String(), "\"")
	}

	secretJson, err := getSecret(clusterId, project, secretName)
	if err != nil {
		return err
	}

	tokenEncoded := strings.Trim(secretJson.Path("data.token").String(), "\"")
	encodedTokenData, err := base64.StdEncoding.DecodeString(tokenEncoded)

	if err != nil {
		log.Println(err.Error())
		return errors.New(genericAPIError)
	}

	// Call the WZU backend
	command := newJenkinsCredentialsCommand{
		OrganizationKey: organizationKey,
		Description:     fmt.Sprintf("OpenShift Deployer - cluster: %v, project: %v, service-account: %v", clusterId, project, serviceaccount),
		Secret:          string(encodedTokenData),
	}
	if err := callWZUBackend(command); err != nil {
		return err
	}

	return nil
}
