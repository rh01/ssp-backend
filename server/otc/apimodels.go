package otc

import (
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
)

type NewECSCommand struct {
	ECSName            string `json:"ecsName"`
	AvailabilityZone   string `json:"availabilityZone"`
	FlavorName         string `json:"flavorName"`
	ImageId            string `json:"imageId"`
	Billing            string `json:"billing"`
	PublicKey          string `json:"publicKey"`
	RootVolumeTypeId   string `json:"rootVolumeTypeId"`
	RootDiskSize       int    `json:"rootDiskSize"`
	SystemVolumeTypeId string `json:"systemVolumeTypeId"`
	SystemDiskSize     int    `json:"systemDiskSize"`
	DataVolumeTypeId   string `json:"dataVolumeTypeId"`
	DataDiskSize       int    `json:"dataDiskSize"`
	MegaId             string `json:"megaId"`
}

type DataDisk struct {
	DiskSize     int    `json:"diskSize"`
	VolumeTypeId string `json:"volumeTypeId"`
}

type FlavorListResponse struct {
	Flavors []Flavor `json:"flavors"`
}

type Flavor struct {
	Name  string `json:"name"`
	VCPUs int    `json:"vcpus"`
	RAM   int    `json:"ram"`
}

type AvailabilityZoneListResponse struct {
	AvailabilityZones []string `json:"availabilityZones"`
}

type ImageListResponse struct {
	Images []Image `json:"images"`
}

type Image struct {
	TrimmedName      string `json:"trimmedName"`
	Name             string `json:"name"`
	Id               string `json:"id"`
	MinDiskGigabytes int    `json:"minDiskGigabytes"`
	MinRAMMegabytes  int    `json:"minRAMMegabytes"`
}

type ECServerListResponse struct {
	Servers []servers.Server `json:"servers"`
}

type VolumeTypesListResponse struct {
	VolumeTypes []VolumeType `json:"volumeTypes"`
}

type VolumeType struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}
