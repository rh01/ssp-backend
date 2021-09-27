package gluster

import "testing"

func init() {
	ExecRunner = TestRunner{}
}

func TestGetVolumeUsage(t *testing.T) {
	output = []string{"    49664    2864 /dev/mapper/vg_mylv_project_pv1"}

	volInfo, _ := getVolumeUsage("gl-project-pv1")

	equals(t, 49664, volInfo.TotalKiloBytes)
	equals(t, 2864, volInfo.UsedKiloBytes)
}

func TestGetVolumeUsage_LongProject(t *testing.T) {
	output = []string{"    49664    2864 /dev/mapper/vg_mylv_project--long_pv1"}

	volInfo, _ := getVolumeUsage("gl-project-long-pv1")

	equals(t, 49664, volInfo.TotalKiloBytes)
	equals(t, 2864, volInfo.UsedKiloBytes)
}

func TestGetVolumeUsage_LongProjectHighNumber(t *testing.T) {
	output = []string{"    49664    2864 /dev/mapper/vg_mylv_project--very--very--long_pv20"}

	volInfo, _ := getVolumeUsage("gl-project-very-very-long-pv20")

	equals(t, 49664, volInfo.TotalKiloBytes)
	equals(t, 2864, volInfo.UsedKiloBytes)
}

func TestCheckVolumeUsage_OK(t *testing.T) {
	output = []string{"    49664    2864 /dev/mapper/vg_mylv_project_pv1"}

	err := checkVolumeUsage("gl-project-pv1", "20")
	ok(t, err)
}

func TestCheckVolumeUsage_Error(t *testing.T) {
	output = []string{"    49664    49555 /dev/mapper/vg_mylv_project_pv1"}

	err := checkVolumeUsage("gl-project-pv1", "20")
	assert(t, err != nil, "Should return error as bigger than threshold")
}
