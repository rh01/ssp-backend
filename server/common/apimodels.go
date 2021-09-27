package common

import "time"
import "github.com/aws/aws-sdk-go/service/ec2"

const ConfigNotSetError = "This feature hasn't been configured correctly. Please contact the CLP Team"

type ProjectName struct {
	Project string `json:"project"`
}

type OpenshiftBase struct {
	Project   string `json:"project"`
	ClusterId string `json:"clusterid"`
}

type NewVolumeCommand struct {
	OpenshiftBase
	Size         string `json:"size"`
	PvcName      string `json:"pvcName"`
	Mode         string `json:"mode"`
	Technology   string `json:"technology"`
	StorageClass string `json:"storageclass"`
}

type FixVolumeCommand struct {
	OpenshiftBase
}

type GrowVolumeCommand struct {
	ClusterId string `json:"clusterid"`
	NewSize   string `json:"newSize"`
	PvName    string `json:"pvName"`
}

type NewProjectCommand struct {
	OpenshiftBase
	Billing string `json:"billing"`
	MegaId  string `json:"megaId"`
}

type NewTestProjectCommand struct {
	OpenshiftBase
}

type EditLogseneBillingDataCommand struct {
	OpenshiftBase
	Billing string `json:"billing"`
}

type UpdateProjectInformationCommand struct {
	OpenshiftBase
	Billing string `json:"billing"`
	MegaID  string `json:"megaid"`
}

type AddProjectAdminCommand struct {
	OpenshiftBase
	Username string `json:"username"`
}

type CreateLogseneAppCommand struct {
	AppName      string `json:"appName"`
	DiscountCode string `json:"discountCode"`
	EditSematextPlanCommand
	UpdateProjectInformationCommand
}

type EditSematextPlanCommand struct {
	PlanId int `json:"planId"`
	Limit  int `json:"limit"`
}

type EditQuotasCommand struct {
	OpenshiftBase
	CPU    int `json:"cpu"`
	Memory int `json:"memory"`
}

type NewServiceAccountCommand struct {
	OpenshiftBase
	ServiceAccount  string `json:"serviceAccount"`
	OrganizationKey string `json:"organizationKey"`
}

type NewPullSecretCommand struct {
	OpenshiftBase
	Username string
	Password string
}

type CreateSnapshotCommand struct {
	InstanceId  string `json:"instanceId"`
	VolumeId    string `json:"volumeId"`
	Description string `json:"description"`
	Account     string `json:"account"`
}

type WorkflowCommand struct {
	UserInputValues []WorkflowKeyValue `json:"userInputValues"`
}

type WorkflowKeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type WorkflowJob struct {
	JobId     int               `json:"jobId"`
	JobStatus WorkflowJobStatus `json:"jobStatus"`
}

type WorkflowJobStatus struct {
	JobStatus                 string                    `json:"jobStatus"`
	ErrorMessage              string                    `json:"errorMessage"`
	ReturnParameters          []WorkflowKeyValue        `json:"returnParameters"`
	WorkflowExecutionProgress WorkflowExecutionProgress `json:"workflow-execution-progress"`
}

type WorkflowExecutionProgress struct {
	CurrentCommandIndex float64 `json:"current-command-index"`
	CommandsNumber      float64 `json:"commands-number"`
}

type ApiResponse struct {
	Message string `json:"message"`
}

type SnapshotApiResponse struct {
	Message  string       `json:"message"`
	Snapshot ec2.Snapshot `json:"snapshot"`
}

type NewVolumeApiResponse struct {
	Message string            `json:"message"`
	Data    NewVolumeResponse `json:"data"`
}

type BucketListResponse struct {
	Buckets []Bucket `json:"buckets"`
}

type Bucket struct {
	Name    string `json:"name"`
	Account string `json:"account"`
}

type NewVolumeResponse struct {
	PvName string
	Server string
	Path   string
	JobId  int
}

type InstanceListResponse struct {
	Instances []Instance `json:"instances"`
}

type Instance struct {
	Name             string          `json:"name"`
	InstanceId       string          `json:"instanceId"`
	InstanceType     string          `json:"instanceType"`
	ImageId          string          `json:"imageId"`
	ImageName        string          `json:"imageName"`
	LaunchTime       *time.Time      `json:"launchTime"`
	State            string          `json:"state"`
	PrivateIpAddress string          `json:"privateIpAddress"`
	Account          string          `json:"account"`
	Snapshots        []*ec2.Snapshot `json:"snapshots"`
	Volumes          []Volume        `json:"volumes"`
	Tags             []*ec2.Tag      `json:"tags"`
}

type Volume struct {
	DeviceName string `json:"deviceName"`
	VolumeId   string `json:"volumeId"`
}

type S3CredentialsResponse struct {
	Username    string `json:"username"`
	AccessKeyID string `json:"accesskeyid"`
	SecretKey   string `json:"secretkey"`
	Password    string `json:"password"`
}

type AdminList struct {
	Admins []string `json:"admins"`
}

type SematextAppList struct {
	AppId         int     `json:"appId"`
	Name          string  `json:"name"`
	PlanName      string  `json:"planName"`
	UserRole      string  `json:"userRole"`
	IsFree        bool    `json:"isFree"`
	PricePerMonth float64 `json:"pricePerMonth"`
	BillingInfo   string  `json:"billingInfo"`
}

type SematextLogsenePlan struct {
	PlanId                     int     `json:"planId"`
	Name                       string  `json:"name"`
	IsFree                     bool    `json:"isFree"`
	PricePerMonth              float64 `json:"pricePerMonth"`
	DefaultDailyMaxLimitSizeMb float64 `json:"defaultDailyMaxLimitSizeMb"`
}

type NewS3BucketCommand struct {
	ProjectName
	BucketName string `json:"bucketname"`
	Billing    string `json:"billing"`
	Stage      string `json:"stage"`
}

type NewS3UserCommand struct {
	UserName   string `json:"username"`
	IsReadonly bool   `json:"isReadonly"`
}

type JsonPatch struct {
	Operation string      `json:"op"`
	Path      string      `json:"path"`
	Value     interface{} `json:"value"`
}
