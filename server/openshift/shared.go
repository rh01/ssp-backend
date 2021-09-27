package openshift

import (
	"crypto/tls"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Jeffail/gabs/v2"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/common"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
	"github.com/gin-gonic/gin"
)

const (
	genericAPIError         = "Error when calling the OpenShift API. Please open a Jira issue"
	wrongAPIUsageError      = "Invalid api call - parameters did not match to method definition"
	testProjectDeletionDays = "30"
)

// RegisterRoutes registers the routes for OpenShift
func RegisterRoutes(r *gin.RouterGroup) {
	// OpenShift
	r.POST("/ose/project", newProjectHandler)
	r.GET("/ose/projects", getProjectsHandler)
	r.GET("/ose/project/admins", getProjectAdminsHandler)
	r.POST("/ose/project/admins", addProjectAdminHandler)
	r.POST("/ose/testproject", newTestProjectHandler)
	r.POST("/ose/serviceaccount", newServiceAccountHandler)
	r.GET("/ose/project/info", getProjectInformationHandler)
	r.POST("/ose/project/info", updateProjectInformationHandler)
	r.GET("/ose/quotas", getQuotasHandler)
	r.POST("/ose/quotas", editQuotasHandler)
	r.POST("/ose/secret/pull", newPullSecretHandler)

	// Volumes (Gluster and NFS)
	r.POST("/ose/volume", newVolumeHandler)
	r.POST("/ose/volume/grow", growVolumeHandler)
	r.POST("/ose/volume/gluster/fix", fixVolumeHandler)
	// Get job status for NFS volumes because it takes a while
	r.GET("/ose/volume/jobs", jobStatusHandler)
	r.GET("/ose/clusters", clustersHandler)
}

func getProjectAdminsAndOperators(clusterId, project string) ([]string, []string, error) {
	adminRoleBinding, err := getAdminRoleBinding(clusterId, project)
	if err != nil {
		return nil, nil, err
	}

	var admins []string
	hasOperatorGroup := false
	for _, g := range adminRoleBinding.Path("groupNames").Children() {
		if strings.ToLower(g.Data().(string)) == "operator" {
			hasOperatorGroup = true
		}
	}
	for _, u := range adminRoleBinding.Path("userNames").Children() {
		admins = append(admins, strings.ToLower(u.Data().(string)))
	}

	var operators []string
	if hasOperatorGroup {
		// Going to add the operator group to the admins
		json, err := getOperatorGroup(clusterId)
		if err != nil {
			return nil, nil, err
		}

		for _, u := range json.Path("users").Children() {
			operators = append(operators, strings.ToLower(u.Data().(string)))
		}
	}
	// remove duplicates because admins are added two times:
	// lowercase and uppercase
	return common.RemoveDuplicates(admins), operators, nil
}

func checkAdminPermissions(clusterId, username, project string) error {
	// Check if user has admin-access
	hasAccess := false
	admins, operators, err := getProjectAdminsAndOperators(clusterId, project)
	if err != nil {
		return err
	}

	username = strings.ToLower(username)

	// Access for admins
	for _, a := range admins {
		if username == a {
			hasAccess = true
		}
	}

	// Access for operators
	for _, o := range operators {
		if username == o {
			hasAccess = true
		}
	}

	if hasAccess {
		return nil
	}

	return fmt.Errorf("You don't have admin permissions on the project: %v. The following users have admin permissions: %v", project, strings.Join(admins, ", "))
}

func getOperatorGroup(clusterId string) (*gabs.Container, error) {
	resp, err := getOseHTTPClient("GET", clusterId, "apis/user.openshift.io/v1/groups/operator", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	json, err := gabs.ParseJSONBuffer(resp.Body)
	if err != nil {
		log.Println("error parsing body of response:", err)
		return nil, errors.New(genericAPIError)
	}

	return json, nil
}

func getAdminRoleBinding(clusterId, project string) (*gabs.Container, error) {
	resp, err := getOseHTTPClient("GET", clusterId, "apis/rbac.authorization.k8s.io/v1/namespaces/"+project+"/rolebindings", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		log.Println("Project was not found", project)
		return nil, errors.New("Das Projekt existiert nicht")
	}
	if resp.StatusCode == 403 {
		log.Println("Cannot list RoleBindings: Forbidden")
		return nil, errors.New(genericAPIError)
	}
	json, err := gabs.ParseJSONBuffer(resp.Body)
	if err != nil {
		log.Println("error parsing body of response:", err)
		return nil, errors.New(genericAPIError)
	}
	var adminRoleBinding *gabs.Container
	var userNames []string
	var groupNames []string
	for _, role := range json.S("items").Children() {
		if role.Path("roleRef.name").Data().(string) == "admin" {
			if adminRoleBinding == nil {
				adminRoleBinding = role
			}
			for _, name := range role.Path("subjects").Children() {
				userNames = append(userNames, strings.ToLower(name.Path("name").Data().(string)))
			}
			for _, name := range role.Path("groupNames").Children() {
				groupNames = append(groupNames, strings.ToLower(name.Data().(string)))
			}
		}
	}

	userNames = common.RemoveDuplicates(userNames)
	adminRoleBinding.Array("userNames")
	for _, name := range userNames {
		adminRoleBinding.ArrayAppend(name, "userNames")
	}
	groupNames = common.RemoveDuplicates(groupNames)
	adminRoleBinding.Array("groupNames")
	for _, name := range groupNames {
		adminRoleBinding.ArrayAppend(name, "groupNames")
	}

	return adminRoleBinding, nil
}

func getOseHTTPClient(method string, clusterId string, endURL string, body io.Reader) (*http.Response, error) {
	cluster, err := getOpenshiftCluster(clusterId)
	if err != nil {
		return nil, err
	}

	token := cluster.Token
	if token == "" {
		log.Printf("WARNING: Cluster token not found. Please see README for more details. ClusterId: %v", clusterId)
		return nil, errors.New(common.ConfigNotSetError)
	}
	base := cluster.URL
	if base == "" {
		log.Printf("WARNING: Cluster URL not found. Please see README for more details. ClusterId: %v", clusterId)
		return nil, errors.New(common.ConfigNotSetError)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	req, _ := http.NewRequest(method, base+"/"+endURL, body)

	log.Debugf("Calling %v", req.URL.String())

	req.Header.Add("Authorization", "Bearer "+token)

	if method == "PATCH" {
		req.Header.Set("Content-Type", "application/json-patch+json")
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error from server: ", err.Error())
		return nil, errors.New(genericAPIError)
	}
	return resp, nil
}

func getWZUBackendClient(method string, endUrl string, body io.Reader) (*http.Response, error) {
	cfg := config.Config()
	wzuBackendUrl := cfg.GetString("wzubackend_url")
	wzuBackendSecret := cfg.GetString("wzubackend_secret")
	if wzuBackendUrl == "" || wzuBackendSecret == "" {
		log.Println("Env variable 'wzuBackendUrl' and 'WZUBACKEND_SECRET' must be specified")
		return nil, errors.New(common.ConfigNotSetError)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, _ := http.NewRequest(method, wzuBackendUrl+"/"+endUrl, body)

	log.Debugf("Calling %v", req.URL.String())

	req.SetBasicAuth("CLOUD_SSP", wzuBackendSecret)

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error from server: ", err.Error())
		return nil, errors.New(genericAPIError)
	}

	return resp, nil
}

func getGlusterHTTPClient(clusterId string, url string, body io.Reader) (*http.Response, error) {
	cluster, err := getOpenshiftCluster(clusterId)
	if err != nil {
		return nil, err
	}

	if cluster.GlusterApi == nil {
		log.Printf("WARNING: GlusterApi is not configured for cluster %v", clusterId)
		return nil, errors.New(common.ConfigNotSetError)
	}

	apiUrl := cluster.GlusterApi.URL
	apiSecret := cluster.GlusterApi.Secret

	if apiUrl == "" || apiSecret == "" {
		log.Printf("WARNING: Gluster url or secret not found. Please see README for more details. ClusterId: %v", clusterId)
		return nil, errors.New(common.ConfigNotSetError)
	}

	client := &http.Client{}
	req, _ := http.NewRequest("POST", fmt.Sprintf("%v/%v", apiUrl, url), body)

	log.Debugf("Calling %v", req.URL.String())

	req.SetBasicAuth("GLUSTER_API", apiSecret)

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error from server: ", err.Error())
		return nil, errors.New(genericAPIError)
	}

	return resp, nil
}

func getNfsHTTPClient(method, clusterId, apiPath string, body io.Reader) (*http.Response, error) {
	cluster, err := getOpenshiftCluster(clusterId)
	if err != nil {
		return nil, err
	}

	if cluster.NfsApi == nil {
		log.Printf("WARNING: NfsApi is not configured for cluster %v", clusterId)
		return nil, errors.New(common.ConfigNotSetError)
	}
	apiUrl := cluster.NfsApi.URL
	apiSecret := cluster.NfsApi.Secret
	nfsProxy := cluster.NfsApi.Proxy

	if apiUrl == "" || apiSecret == "" || nfsProxy == "" {
		log.Printf("WARNING: incorrect NFS config. Please see README for more details. ClusterId: %v", clusterId)
		return nil, errors.New(common.ConfigNotSetError)
	}

	// Create http client with proxy:
	// https://blog.abhi.host/blog/2016/02/27/golang-creating-https-connection-via/
	proxyURL, err := url.Parse(nfsProxy)
	if err != nil {
		log.Printf(err.Error())
	}

	transport := http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: &transport}
	req, err := http.NewRequest(method, fmt.Sprintf("%v/%v", apiUrl, apiPath), body)
	if err != nil {
		log.Printf(err.Error())
	}

	log.Debugf("Calling %v", req.URL.String())

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth("sbb_openshift", apiSecret)

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error from server: ", err.Error())
		return nil, errors.New(genericAPIError)
	}

	return resp, err
}

func newObjectRequest(kind string, name string, apiVersion string) *gabs.Container {
	json := gabs.New()

	json.Set(kind, "kind")
	json.Set(apiVersion, "apiVersion")
	json.SetP(name, "metadata.name")

	return json
}

func generateID() string {
	var result string
	// All the possible characters in the ID
	chrs := "0123456789abcdefghijklmnopqrstuvwxyz"
	len := int64(len(chrs))
	// Constant to subtract so the generated ID is shorter
	// Value is Unix timestamp at release of this function
	subtract := int64(1543222754)
	// We use unix timestamp because it increments each second
	// The time is not important
	unix := time.Now().Unix() - subtract
	for unix > 0 {
		result = string(chrs[unix%len]) + result
		// division without remainder
		unix = unix / len
	}
	return result
}
