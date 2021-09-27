package models

// CreateLVCommand is the model for a new LV
type CreateLVCommand struct {
	Size       string `json:"size"`
	MountPoint string `json:"mountPoint"`
	LvName     string `json:"lvName"`
}

// CreateVolumeCommand is the model for a new gluster volume
type CreateVolumeCommand struct {
	Project string `json:"project"`
	Size    string `json:"size"`
}

// GrowVolumeCommand is the model to grow an existing gluster volume
type GrowVolumeCommand struct {
	PvName  string `json:"pvName"`
	NewSize string `json:"newSize"`
}

type DeleteVolumeCommand struct {
	LvName string `json:"lvName"`
}

// VolInfo is the response model for the volume info endpoint
type VolInfo struct {
	TotalKiloBytes int `json:"totalKiloBytes"`
	UsedKiloBytes  int `json:"usedKiloBytes"`
}
