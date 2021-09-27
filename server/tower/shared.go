package tower

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/common"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/otc"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	wrongAPIUsageError = "Ungültiger API-Aufruf: Die Argumente stimmen nicht mit der definition überein. Bitte erstelle ein Ticket"
	genericAPIError    = "Fehler beim Aufruf der Ansible Tower API. Bitte erstelle ein Ticket"
)

func RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/tower/jobs/:job/stdout", getJobOutputHandler)
	r.GET("/tower/jobs/:job", getJobHandler)
	r.GET("/tower/jobs", getJobsHandler)
	r.GET("/tower/job_templates/:jobTemplate/getDetails", getJobTemplateGetDetailsHandler)
	r.POST("/tower/job_templates/:jobTemplate/launch", postJobTemplateLaunchHandler)
}

func postJobTemplateLaunchHandler(c *gin.Context) {
	username := common.GetUserName(c)
	jobTemplate := c.Param("jobTemplate")

	request, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAPIError})
		return
	}
	json, err := gabs.ParseJSON(request)
	if err != nil {
		log.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAPIError})
		return
	}
	job, err := launchJobTemplate(jobTemplate, json, username)
	if err != nil {
		log.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAPIError})
		return
	}
	c.JSON(http.StatusOK, job)
}

func launchJobTemplate(jobTemplate string, json *gabs.Container, username string) (string, error) {
	// Check if the user is allowed to execute this jobTemplate.
	// This also checks if the jobTemplate is whitelisted (see sample config)
	if err := checkPermissions(jobTemplate, json, username); err != nil {
		return "", err
	}

	// Remove extra_vars that the user is not allowed to set.
	json = removeBlacklistedParameters(json)

	// Overwrite/set the username, this is mostly used for email notifications and
	// for filtering jobs in the SSP (list all jobs with one username)
	json.SetP(username, "extra_vars.custom_tower_user_name")
	log.Printf("%+v", json)

	// Add an Ansible skip tag for filtering in the SSP.
	// The skip tag normally skips any Ansible code with this tag,
	// but since there is none, it is ignored.
	// We need this because filtering on extra_vars is not possible
	// and artifacts only appear when the job is done.
	json.SetP("ssp_filter_"+username, "skip_tags")

	resp, err := getTowerHTTPClient("POST", "job_templates/"+jobTemplate+"/launch/", bytes.NewReader(json.Bytes()))

	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	json, err = gabs.ParseJSON(body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusBadRequest {
		// Should never happen. This means the SSP and Tower send/expect different variables
		errs := "Error from Ansible Tower:"
		for _, err := range json.Path("variables_needed_to_start").Children() {
			errs += ", " + err.Data().(string)
		}
		return "", fmt.Errorf(string(errs))
	}
	return string(body), nil
}

func getJobTemplateGetDetailsHandler(c *gin.Context) {
	username := common.GetUserName(c)
	jobTemplate := c.Param("jobTemplate")

	details, err := getJobTemplateDetails(jobTemplate, username)
	if err != nil {
		log.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAPIError})
		return
	}
	c.JSON(http.StatusOK, details)
}

func getJobTemplateDetails(jobTemplate string, username string) (string, error) {
	// Check if the user is allowed to execute this jobTemplate.
	// This also checks if the jobTemplate is whitelisted (see sample config)
	if err := checkPermissions(jobTemplate, nil, username); err != nil {
		return "", err
	}

	resp, err := getTowerHTTPClient("GET", "job_templates/"+jobTemplate+"/survey_spec/", nil)

	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusBadRequest {
		// Should never happen
		return "", fmt.Errorf("Error from Ansible Tower")
	}
	details, err := gabs.ParseJSON(body)
	if err != nil {
		return "", err
	}

	err = addSpecsMap(details)
	if err != nil {
		return "", err
	}

	return details.String(), nil
}

func addSpecsMap(details *gabs.Container) error {
	// rearranging specs as a hashmap for better navigation in frontend
	for index, spec := range details.Path("spec").Children() {
		variableName, _ := spec.Search("variable").Data().(string)
		for k, v := range spec.ChildrenMap() {
			_, err := details.Set(v.Data(), "specsMap", variableName, k)
			if err != nil {
				return err
			}
		}
		details.Set(index, "specsMap", variableName, "index")
	}
	return nil
}

type jobTemplateConfig struct {
	ID       string
	Validate string
}

func removeBlacklistedParameters(json *gabs.Container) *gabs.Container {
	cfg := config.Config()
	var blacklist []string
	if err := cfg.UnmarshalKey("tower.parameter_blacklist", &blacklist); err != nil {
		log.Warn("No Ansible-Tower parameter blacklist found")
	}
	for _, p := range blacklist {
		if json.Exists("extra_vars", p) {
			json.Delete("extra_vars", p)
			log.WithFields(log.Fields{
				"parameter": p,
			}).Warn("Removed blacklisted parameter!")
		}
	}
	return json
}

func checkPermissions(jobTemplate string, json *gabs.Container, username string) error {
	cfg := config.Config()

	jobTemplateConfigs := []jobTemplateConfig{}
	if err := cfg.UnmarshalKey("tower.job_templates", &jobTemplateConfigs); err != nil {
		return err
	}
	// Check if the template id is whitelisted in the config file (see sample config)
	for _, t := range jobTemplateConfigs {
		if t.ID != jobTemplate {
			continue
		}
		// This is an optional setting in the configfile (see sample config)
		// It means that additional checks are needed. This is mostly done
		// by calling an external service/package.
		if t.Validate != "" {
			if err := checkServicePermissions(t, json, username); err != nil {
				return err
			}
		}
		log.Printf("Job template %v allowed for %v", jobTemplate, username)
		return nil
	}
	return fmt.Errorf("Username %v tried to launch job template %v. Not in allowed job_templates", username, jobTemplate)
}

// This function is only executed if "validate" is specified in the configfile
// There can be multiple validations (see below), if the specified validation
// doesn't exist in the below code, then the check will fail.
func checkServicePermissions(template jobTemplateConfig, json *gabs.Container, username string) error {
	// Validate the uos_group metadata on the server, that is being modified/deleted.
	// Permission only has to be checked if the server already exists.
	if template.Validate == "metadata.uos_group" {
		// To check the "metadata.uos_group" field we need to get the server from OTC
		// We mostly do this by hostname, because the ID is not human readable and
		// this data mostly comes from the Tower Survey or SSP.
		//
		// **Notes for future contributors:**
		// At the moment the tenant is evaluated with the hostname and there is only the managed project.
		// When there are more tenants/projects it might be necessary to somehow evaluate
		// which tenant/project the server hostname belongs to. This could be achieved by parsing the
		// job templates name (from Tower), if these are consistent. Another possibility would be to
		// add tenant and project fields to every job_template in the config file (see jobTemplateConfig struct).
		servername := json.Path("extra_vars.unifiedos_hostname").Data().(string)
		// this function gets the server data and validates the groups of username against the metadata
		if err := otc.ValidatePermissionsByHostname(servername, username); err != nil {
			return err
		}
		// If there is no error, then the user has permission
		return nil
	}
	// Fail if the validation is not defined above or there is a typo in the configuration
	return fmt.Errorf("No existing validation matches: %v Check the configuration", template.Validate)
}

func getJobOutputHandler(c *gin.Context) {
	job := c.Param("job")
	resp, err := getTowerHTTPClient("GET", "jobs/"+job+"/stdout/?format=html", nil)
	if err != nil {
		log.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAPIError})
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAPIError})
		return
	}

	c.JSON(http.StatusOK, string(body))
}

func getJobHandler(c *gin.Context) {
	job := c.Param("job")
	resp, err := getTowerHTTPClient("GET", "jobs/"+job, nil)
	if err != nil {
		log.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAPIError})
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAPIError})
		return
	}

	c.JSON(http.StatusOK, string(body))
}

func getJobsHandler(c *gin.Context) {
	username := common.GetUserName(c)
	// We need to first get the finished jobs and then the failed/running jobs, because the Tower-API
	// doesn't allow filtering by extra_vars (as far as I know).
	finishedJobs, err := getFinishedJobs(username)
	if err != nil {
		log.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAPIError})
		return
	}
	failedOrRunningJobs, err := getFailedOrRunningJobs(username)
	if err != nil {
		log.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAPIError})
		return
	}
	finishedJobs.Merge(failedOrRunningJobs)

	c.JSON(http.StatusOK, finishedJobs.S("results").String())
}

// TODO: wait a few weeks, switch to skip tag filtering and remove this code
func getFinishedJobs(username string) (*gabs.Container, error) {
	// Get all the jobs that have artifacts which contain the username. This could produce a few
	// false-positives in the future.
	resp, err := getTowerHTTPClient("GET", "jobs/?order_by=-created&artifacts__contains="+username, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return gabs.ParseJSON(body)
}

// TODO: wait a few weeks, switch to skip tag filtering and remove this code
func getFailedOrRunningJobs(username string) (*gabs.Container, error) {
	// Get all the failed/running jobs (of all users, because we cannot filter by extra_vars
	// and artifacts are not available yet) and then loop through and only keep
	// if custom_tower_user_name is set.
	resp, err := getTowerHTTPClient("GET", "jobs/?order_by=-created&or__status=failed&or__finished__isnull=true", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	jobs, err := gabs.ParseJSON(body)
	if err != nil {
		return nil, err
	}
	jsonObj := gabs.New()
	// Ugly hack to filter on extra_vars.custom_tower_user_name
	// Because the tower api doesn't allow filtering on custom_vars
	// custom_vars is an escaped json string
	for _, job := range jobs.S("results").Children() {
		extra_vars, err := gabs.ParseJSON([]byte(job.S("extra_vars").Data().(string)))
		if err != nil {
			log.Error(err)
			continue
		}
		// Can be nil, if the value doesn't exist
		ctun := extra_vars.S("custom_tower_user_name").Data()
		if ctun != nil && ctun.(string) == username {
			jsonObj.ArrayAppend(job.Data(), "results")
		}
	}
	return jsonObj, nil
}

func getTowerHTTPClient(method string, urlPart string, body io.Reader) (*http.Response, error) {
	cfg := config.Config()
	baseUrl := cfg.GetString("tower.base_url")
	if baseUrl == "" {
		log.Error("Env variables 'TOWER_BASE_URL' must be specified")
		return nil, errors.New(common.ConfigNotSetError)
	}

	username := cfg.GetString("tower.username")
	password := cfg.GetString("tower.password")
	if username == "" || password == "" {
		log.Error("Env variables 'TOWER_USERNAME' and 'TOWER_PASSWORD' must be specified")
		return nil, errors.New(common.ConfigNotSetError)
	}

	if !strings.HasSuffix(baseUrl, "/") {
		baseUrl += "/"
	}

	client := &http.Client{}
	req, _ := http.NewRequest(method, baseUrl+urlPart, body)
	req.SetBasicAuth(username, password)

	log.Debugf("Calling %v", req.URL.String())

	req.Header.Add("Content-Type", "application/json")

	return client.Do(req)
}
