package aws

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/gin-gonic/gin"
)

const (
	ec2ListError  = "Instances can't be listed. Please open a ticket"
	ec2StartError = "Instances can't be started. Please open a ticket"
	ec2StopError  = "Instances can't be stopped. Please open a ticket"
)

func listEC2InstancesHandler(c *gin.Context) {
	username := common.GetUserName(c)

	log.Println(username + " lists EC2 Instances")

	instances, err := listEC2InstancesByUsername(username)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
	} else {
		c.JSON(http.StatusOK, instances)
	}
}

func deleteEC2InstanceSnapshotHandler(c *gin.Context) {
	username := common.GetUserName(c)
	snapshotid := c.Param("snapshotid")
	account := c.Param("account")
	err := deleteSnapshot(snapshotid, account)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAwsAPIError})
		return
	}
	log.Println(username + " deleted snapshot " + snapshotid)
	c.JSON(http.StatusOK, common.ApiResponse{Message: "Snapshot has been deleted"})
}

func createEC2InstanceSnapshotHandler(c *gin.Context) {
	username := common.GetUserName(c)
	var data common.CreateSnapshotCommand
	if c.BindJSON(&data) == nil {
		snapshot, err := createSnapshot(data.VolumeId, data.InstanceId, data.Description, data.Account)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAwsAPIError})
			return
		}
		log.Println(username + " snapshots volume " + data.VolumeId + " in instance " + data.InstanceId)
		c.JSON(http.StatusOK, common.SnapshotApiResponse{Message: "Successfully created snapshot: " + data.Description, Snapshot: *snapshot})
		return
	}
	c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
}

func setEC2InstanceStateHandler(c *gin.Context) {
	username := common.GetUserName(c)
	instanceid := c.Param("instanceid")
	state := c.Param("state")
	log.Print(username + " requested instance " + instanceid + " to " + state)
	instance, err := getInstance(instanceid, username)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		return
	}
	account := instance.Account

	switch state {
	case "start":
		res, err := startEC2Instance(instanceid, username, account)
		if err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	case "stop":
		res, err := stopEC2Instance(instanceid, username, account)
		if err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	default:
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
	}
}

func deleteSnapshot(snapshotid string, account string) error {
	svc, err := GetEC2ClientForAccount(account)
	if err != nil {
		return err
	}

	_, err = svc.DeleteSnapshot(&ec2.DeleteSnapshotInput{SnapshotId: aws.String(snapshotid)})
	if err != nil {
		log.Println("Error creating snapshot (CreateSnapshot API call): " + err.Error())
		return err
	}
	return nil
}

func createSnapshot(volumeid string, instanceid string, description string, account string) (*ec2.Snapshot, error) {
	tags, err := getTags(volumeid, account)
	if err != nil {
		log.Println("Error getting tags: " + err.Error())
		return nil, err
	}
	deviceName, err := getDeviceName(volumeid, account)
	if err != nil {
		return nil, err
	}
	tags = appendTag(tags, "instance_id", instanceid)
	tags = appendTag(tags, "devicename", deviceName)
	input := &ec2.CreateSnapshotInput{
		Description: aws.String(description),
		VolumeId:    aws.String(volumeid),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String("snapshot"),
				Tags:         tags,
			},
		},
	}

	svc, err := GetEC2ClientForAccount(account)
	if err != nil {
		log.Println("Error getting EC2 client: " + err.Error())
		return nil, err
	}

	snapshot, err := svc.CreateSnapshot(input)
	if err != nil {
		log.Println("Error creating snapshot (CreateSnapshot API call): " + err.Error())
		return nil, err
	}
	return snapshot, nil
}

func getInstance(instanceid string, username string) (*common.Instance, error) {
	instances, err := listEC2InstancesByUsername(username)
	if err != nil {
		return nil, err
	}
	for _, instance := range instances.Instances {
		if instance.InstanceId == instanceid {
			return &instance, nil
		}
	}
	log.Println("Could not find an instance with id: " + instanceid)
	return nil, errors.New(ec2ListError)
}

func startEC2Instance(instanceid string, username string, account string) (*common.Instance, error) {
	input := &ec2.StartInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceid),
		},
	}

	svc, err := GetEC2ClientForAccount(account)
	if err != nil {
		log.Println("Error getting EC2 client: " + err.Error())
		return nil, errors.New(ec2StartError)
	}

	_, err = svc.StartInstances(input)
	if err != nil {
		log.Println("Error starting EC2 instance (StartInstances API call): " + err.Error())
		return nil, errors.New(ec2StartError)
	}

	filters := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("instance-id"),
				Values: []*string{
					aws.String(instanceid),
				},
			},
		},
	}
	err = svc.WaitUntilInstanceRunning(filters)
	if err != nil {
		log.Println("Error waiting for EC2 instance to start: " + err.Error())
		return nil, errors.New(ec2StartError)
	}
	result, err := getInstance(instanceid, username)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func stopEC2Instance(instanceid string, username string, account string) (*common.Instance, error) {
	input := &ec2.StopInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceid),
		},
	}

	svc, err := GetEC2ClientForAccount(account)
	if err != nil {
		log.Println("Error getting EC2 client: " + err.Error())
		return nil, errors.New(ec2StopError)
	}

	_, err = svc.StopInstances(input)
	if err != nil {
		log.Println("Error stopping EC2 instance (StopInstances API call): " + err.Error())
		return nil, errors.New(ec2StopError)
	}

	filters := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("instance-id"),
				Values: []*string{
					aws.String(instanceid),
				},
			},
		},
	}
	err = svc.WaitUntilInstanceStopped(filters)
	if err != nil {
		log.Println("Error waiting for EC2 instance to stop: " + err.Error())
		return nil, errors.New(ec2StopError)
	}

	result, err := getInstance(instanceid, username)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func listEC2InstancesByUsername(username string) (*common.InstanceListResponse, error) {
	result := common.InstanceListResponse{
		Instances: []common.Instance{},
	}
	nonprodInstances, err := listEC2InstancesByUsernameForAccount(username, accountNonProd)
	if err != nil {
		return nil, err
	}
	prodInstances, err := listEC2InstancesByUsernameForAccount(username, accountProd)
	if err != nil {
		return nil, err
	}

	result.Instances = append(result.Instances, nonprodInstances...)
	result.Instances = append(result.Instances, prodInstances...)

	return &result, nil
}

func listEC2InstancesByUsernameForAccount(username string, account string) ([]common.Instance, error) {
	instances := []common.Instance{}
	filters := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag:Owner"),
				Values: []*string{
					// Case insensitive filter
					aws.String("*" + strings.ToLower(username) + "*"),
					aws.String("*" + strings.ToUpper(username) + "*"),
				},
			},
		},
	}

	svc, err := GetEC2ClientForAccount(account)
	if err != nil {
		return nil, err
	}

	result, err := svc.DescribeInstances(filters)
	if err != nil {
		log.Print("Unable to list instances (DescribeInstances API call): " + err.Error())
		return nil, errors.New(ec2ListError)
	}
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			snapshots, _ := listSnapshots(instance, account)
			volumes := listVolumes(instance)
			instances = append(instances, getInstanceStruct(instance, account, snapshots, volumes))
		}
	}

	return instances, nil
}

func listSnapshots(instance *ec2.Instance, account string) ([]*ec2.Snapshot, error) {
	svc, err := GetEC2ClientForAccount(account)
	if err != nil {
		return nil, errors.New(ec2ListError)
	}

	filters := &ec2.DescribeSnapshotsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("tag:instance_id"),
				Values: []*string{
					aws.String(*instance.InstanceId),
				},
			},
		},
	}

	snapshotsOutput, err := svc.DescribeSnapshots(filters)
	if err != nil {
		return nil, err
	}

	// Backward compatibility for snapshots before devicename tag
	// Use devicename from attachment if available or output error
	for _, snapshot := range snapshotsOutput.Snapshots {
		// tag doesn't exist if the snapshot was created before this commit
		if !hasTag(snapshot.Tags, "devicename") {
			// try and get devicename. this works if the original volume
			// is still attached to an ec2 instance.
			// If the device name cannot be found return an error to the user
			devicename, err := getDeviceName(*snapshot.VolumeId, account)
			if err != nil {
				devicename = "Disk name unknown"
			}
			// Append to tag list, so the frontend can display it
			// Snapshots that are created after this change
			// should already contain this tag!
			snapshot.Tags = appendTag(snapshot.Tags, "devicename", devicename)
		}
	}

	return snapshotsOutput.Snapshots, nil
}

// Returns true if tag exists in list
func hasTag(tags []*ec2.Tag, name string) bool {
	for _, tag := range tags {
		if *tag.Key == name {
			return true
		}
	}
	return false
}

func listVolumes(instance *ec2.Instance) []common.Volume {
	volumes := []common.Volume{}
	for _, volume := range instance.BlockDeviceMappings {
		volumes = append(volumes, getVolumeStruct(volume))
	}
	return volumes
}

func getVolumeStruct(volume *ec2.InstanceBlockDeviceMapping) common.Volume {
	return common.Volume{
		DeviceName: *volume.DeviceName,
		VolumeId:   *volume.Ebs.VolumeId,
	}
}

func getDeviceName(volumeId string, account string) (string, error) {
	input := &ec2.DescribeVolumesInput{
		VolumeIds: []*string{
			aws.String(volumeId),
		},
	}

	svc, err := GetEC2ClientForAccount(account)
	if err != nil {
		log.Println("Error getting EC2 client: " + err.Error())
		return "", errors.New(ec2StartError)
	}

	describeVolumesOutput, err := svc.DescribeVolumes(input)
	if err != nil {
		log.Println("Error getting EC2 volumes (DescribeVolumes API call): " + err.Error())
		return "", errors.New(ec2StartError)
	}
	if describeVolumesOutput.Volumes[0].Attachments == nil {
		return "", errors.New("Diskname couldn't be found")
	}
	return *describeVolumesOutput.Volumes[0].Attachments[0].Device, nil
}

func appendTag(tags []*ec2.Tag, name string, value string) []*ec2.Tag {
	if hasTag(tags, name) {
		return tags
	}
	tags = append(tags, &ec2.Tag{Key: aws.String(name), Value: aws.String(value)})
	return tags
}

func getTags(resourceid string, account string) ([]*ec2.Tag, error) {
	input := &ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("resource-id"),
				Values: []*string{
					aws.String(resourceid),
				},
			},
		},
	}

	svc, err := GetEC2ClientForAccount(account)
	if err != nil {
		log.Println("Error getting EC2 client: " + err.Error())
		return nil, err
	}

	describetagsoutput, err := svc.DescribeTags(input)
	if err != nil {
		log.Println("Error getting EC2 tags (DescribeTags API call): " + err.Error())
		return nil, err
	}
	tags := []*ec2.Tag{}
	for _, tagdescription := range describetagsoutput.Tags {
		tags = append(tags, &ec2.Tag{Key: tagdescription.Key, Value: tagdescription.Value})
	}
	return tags, nil
}

func getImageName(imageId string, account string) (*string, error) {
	input := &ec2.DescribeImagesInput{
		ImageIds: []*string{
			aws.String(imageId),
		},
	}

	svc, err := GetEC2ClientForAccount(account)
	if err != nil {
		log.Println("Error getting EC2 client: " + err.Error())
		return nil, err
	}

	describeImagesOutput, err := svc.DescribeImages(input)
	if err != nil {
		log.Println("Error getting EC2 image name (DescribeImages API call): " + err.Error())
		return nil, err
	}
	if len(describeImagesOutput.Images) == 0 {
		// You may not be permitted to view it
		unknownstr := "Unknown"
		return &unknownstr, nil
	}
	return describeImagesOutput.Images[0].Name, nil
}

func getInstanceStruct(instance *ec2.Instance, account string, snapshots []*ec2.Snapshot, volumes []common.Volume) common.Instance {
	var name string
	for _, tag := range instance.Tags {
		if *tag.Key == "Name" {
			name = *tag.Value
			break
		}
	}
	imageName, _ := getImageName(*instance.ImageId, account)

	// there is no privateIp when the instance has been terminated
	var privateIpAddress string
	if instance.PrivateIpAddress != nil {
		privateIpAddress = *instance.PrivateIpAddress
	}
	return common.Instance{
		Name:             name,
		InstanceId:       *instance.InstanceId,
		InstanceType:     *instance.InstanceType,
		ImageId:          *instance.ImageId,
		ImageName:        *imageName,
		LaunchTime:       instance.LaunchTime,
		PrivateIpAddress: privateIpAddress,
		State:            *instance.State.Name,
		Account:          account,
		Snapshots:        snapshots,
		Volumes:          volumes,
		Tags:             instance.Tags,
	}
}
