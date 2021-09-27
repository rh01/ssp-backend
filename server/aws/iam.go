package aws

import (
	"errors"
	"log"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"

	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/common"
)

const (
	genericUserCreationError = "An error occured while creating the user account"
)

// PolicyDocument IAM Policy Document
type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

// StatementEntry IAM Statement Entry
type StatementEntry struct {
	Effect   string
	Action   []string
	Resource []string
}

func validateNewS3User(username string, bucketname string, newuser string, stage string) error {
	if len(username) == 0 {
		return errors.New("Username must be set")
	}
	if len(bucketname) == 0 {
		return errors.New("Bucket name must be set")
	}
	if len(newuser) == 0 {
		return errors.New("Bucket username must be set")
	}

	if (len(newuser) + len(bucketname)) > 63 {
		// http://docs.aws.amazon.com/IAM/latest/UserGuide/reference_iam-limits.html
		return errors.New("Generated user '" + bucketname + "-" + newuser + "' is to long")
	}
	validName := regexp.MustCompile(`^[a-zA-Z0-9\-]+$`).MatchString
	if !validName(bucketname) {
		return errors.New("Username can only contain alphanumeric characters and -")
	}

	svc, err := GetIAMClient(stage)
	if err != nil {
		return err
	}
	result, err := svc.ListUsers(nil)
	if err != nil {
		log.Print("Error while trying to create a new user (ListUsers call): " + err.Error())
		return errors.New(genericUserCreationError)
	}
	// Loop over existing users
	for _, u := range result.Users {
		if *u.UserName == newuser {
			log.Printf("Error, user %v already exists", newuser)
			return errors.New("Error: IAM account " + newuser + " already exists")
		}
	}

	// Make sure the user is allowed to create new IAM users for this bucket
	myBuckets, _ := listS3BucketByUsername(username)
	for _, mybucket := range myBuckets.Buckets {
		if bucketname == mybucket.Name {
			// Everything OK
			return nil
		}
	}
	return errors.New("Bucket " + bucketname + " doesn't exist or you're not allowed to create a Bucket")
}

func createNewS3User(bucketname string, s3username string, stage string, isReadonly bool) (*common.S3CredentialsResponse, error) {
	generatedName := bucketname + "-" + s3username

	svc, err := GetIAMClient(stage)
	if err != nil {
		return nil, err
	}
	usr, err := svc.GetUser(&iam.GetUserInput{
		UserName: aws.String(generatedName),
	})

	if usr != nil && usr.User != nil {
		return nil, errors.New("The user already exists")
	}

	cred := common.S3CredentialsResponse{
		Username: generatedName,
	}
	if errAws, ok := err.(awserr.Error); ok && errAws.Code() == iam.ErrCodeNoSuchEntityException {
		_, err := svc.CreateUser(&iam.CreateUserInput{
			UserName: aws.String(generatedName),
		})

		if err != nil {
			log.Println("CreateUser error in createNewS3User: " + err.Error())
			return nil, errors.New(genericUserCreationError)
		}

		// Create access key
		result, err := svc.CreateAccessKey(&iam.CreateAccessKeyInput{
			UserName: aws.String(generatedName),
		})
		cred.AccessKeyID = *result.AccessKey.AccessKeyId
		cred.SecretKey = *result.AccessKey.SecretAccessKey
	} else {
		log.Println("Failed to create used: ", err.Error())
		return nil, errors.New(genericUserCreationError)
	}

	policy := bucketname
	if isReadonly {
		policy += bucketReadPolicy
	} else {
		policy += bucketWritePolicy
	}

	err = attachIAMPolicyToUser(policy, generatedName, stage)
	if err != nil {
		log.Print("Error while calling attachIAMPolicyToUser: " + err.Error())
		return &cred, errors.New(genericUserCreationError)
	}

	addUserToGroup(generatedName, "S3-Functionuser", stage)

	password, err := getRandomPassword(stage)
	if err != nil {
		log.Print("Error while calling addUserToGroup: " + err.Error())
		return nil, errors.New(genericUserCreationError)
	}
	err = createLoginProfile(generatedName, password, stage)
	if err != nil {
		log.Print("Error while calling createLoginProfile: " + err.Error())
		return nil, errors.New(genericUserCreationError)
	}
	cred.Password = *password

	return &cred, nil
}

func addUserToGroup(user, group, stage string) error {
	svc, err := GetIAMClient(stage)
	if err != nil {
		return err
	}
	input := &iam.AddUserToGroupInput{
		GroupName: &group,
		UserName:  &user,
	}

	_, err = svc.AddUserToGroup(input)
	if err != nil {
		log.Printf("Error while calling AddUserToGroup: %v", err.Error())
		return errors.New(genericUserCreationError)
	}
	return nil
}

func getRandomPassword(stage string) (*string, error) {
	svc, err := GetSecretsmanagerClient(stage)
	if err != nil {
		return nil, err
	}
	output, err := svc.GetRandomPassword(nil)
	if err != nil {
		return nil, err
	}
	return output.RandomPassword, nil
}

func createLoginProfile(username string, password *string, stage string) error {
	svc, err := GetIAMClient(stage)
	if err != nil {
		return err
	}
	input := &iam.CreateLoginProfileInput{
		UserName: &username,
		Password: password,
	}

	_, err = svc.CreateLoginProfile(input)
	if err != nil {
		return err
	}
	return nil
}

func attachIAMPolicyToUser(policyName string, username string, stage string) error {
	svc, err := GetIAMClient(stage)
	if err != nil {
		return err
	}

	// First, figure out the AWS account ID
	result, err := svc.GetUser(nil)
	var accountNumber string
	if err != nil {
		return errors.New("GetUser error in attachIAMPolicyToUser() while trying to determine account ID: " + err.Error())
	}
	re := regexp.MustCompile("[0-9]+")
	accountNumber = re.FindString(*result.User.Arn)

	// Then, attach the policy given to the user
	input := &iam.AttachUserPolicyInput{
		PolicyArn: aws.String("arn:aws:iam::" + accountNumber + ":policy/" + policyName),
		UserName:  aws.String(username),
	}
	_, err = svc.AttachUserPolicy(input)
	if err != nil {
		return errors.New("AttachUserPolicy error: " + err.Error())
	}

	return nil
}
