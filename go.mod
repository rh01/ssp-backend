module github.com/SchweizerischeBundesbahnen/ssp-backend

//replace github.com/gophercloud/gophercloud => github.com/huaweicloud/huaweicloud-sdk-go v1.0.20
replace github.com/gophercloud/gophercloud => github.com/SchweizerischeBundesbahnen/huaweicloud-sdk-go v0.0.0-20200218121541-f9602c8941ee

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/Jeffail/gabs v1.1.1
	github.com/Jeffail/gabs/v2 v2.1.0
	github.com/aws/aws-sdk-go v1.16.30
	github.com/gin-contrib/cors v0.0.0-20190101123304-5e7acb10687f
	github.com/gin-gonic/gin v1.3.0
	github.com/gophercloud/gophercloud v0.0.0-20190328013130-c923f33b1166
	github.com/imdario/mergo v0.3.8
	github.com/jarcoal/httpmock v1.0.4
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/sirupsen/logrus v1.3.0
	github.com/spf13/viper v1.3.1
	golang.org/x/crypto v0.0.0-20191227163750-53104e6ec876
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys v0.0.0-20200106162015-b016eb3dc98e // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/asn1-ber.v1 v1.0.0-20181015200546-f715ec2f112d // indirect
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/ldap.v2 v2.5.1
	gopkg.in/square/go-jose.v2 v2.3.1
)

go 1.13
