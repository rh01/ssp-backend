package gluster

import (
	"testing"

	"github.com/jarcoal/httpmock"
)

func init() {
	ExecRunner = TestRunner{}
}

func TestGrowVolume_Empty(t *testing.T) {
	err := growVolume("", "")
	assert(t, err != nil, "growVolume should throw error if called empty")
}

func TestGrowVolume_WrongSize(t *testing.T) {
	err := growVolume("pv", "101G")
	assert(t, err != nil, "growVolume should throw error if called with wrong size")
}

func TestGrowVolume_WrongSizeMB(t *testing.T) {
	err := growVolume("pv", "1025M")
	assert(t, err != nil, "growVolume should throw error if called with wrong size")
}

func TestGrowVolume(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("POST", "http://192.168.125.236:0/sec/lv/grow",
		httpmock.NewStringResponder(200, ""))

	commands = nil
	output = []string{"Hostname: 192.168.125.236"}
	VgName = "myvg"

	growVolume("pv", "10M")

	// Should call the remote server
	equals(t, 1, httpmock.GetTotalCallCount())

	// Should execute commands locally
	equals(t, "bash -c lvextend -L 10M /dev/myvg/lv_pv", commands[1])
	equals(t, "bash -c xfs_growfs /dev/myvg/lv_pv", commands[2])
}
