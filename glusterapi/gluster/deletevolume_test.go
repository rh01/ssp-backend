package gluster

import (
	"testing"

	"github.com/jarcoal/httpmock"
)

func init() {
	ExecRunner = TestRunner{}
}

func TestGetMountPath_FirstVolume(t *testing.T) {
	BasePath = "/gluster/project"
	mountPath := getMountPath("vol_some-project_pv1")
	equals(t, "/gluster/project/some-project/pv1", mountPath)
}

func TestGetMountPath_SecondVolume(t *testing.T) {
	BasePath = "/gluster/project"
	mountPath := getMountPath("vol_another-project_pv2")
	equals(t, "/gluster/project/another-project/pv2", mountPath)
}

func TestDeleteVolume_Empty(t *testing.T) {
	err := deleteVolume("")
	assert(t, err != nil, "Not all input values provided")
}

func TestDeleteGlusterVolume(t *testing.T) {
	commands = nil
	deleteGlusterVolume("vol_my-project_pv1")
	equals(t, "bash -c gluster volume stop vol_my-project_pv1 --mode=script", commands[0])
	equals(t, "bash -c gluster volume delete vol_my-project_pv1 --mode=script", commands[1])
}

func TestDeleteLvLocally(t *testing.T) {
	commands = nil
	BasePath = "/gluster/project"
	VgName = "vgname"
	deleteLvLocally("vol_my-project_pv1")
	equals(t, "bash -c sed -i '\\#/dev/vgname/lv_my-project_pv1#d' /etc/fstab", commands[0])
	equals(t, "bash -c umount /gluster/project/my-project/pv1", commands[1])
	equals(t, "bash -c lvremove --yes /dev/vgname/lv_my-project_pv1", commands[2])
	equals(t, "bash -c rmdir --parents --ignore-fail-on-non-empty /gluster/project/my-project/pv1", commands[3])
}

func TestDeleteLvOnOtherServers(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("POST", "http://192.168.125.236:0/sec/lv/delete",
		httpmock.NewStringResponder(200, ""))

	output = []string{"Hostname: 192.168.125.236"}

	deleteLvOnOtherServers("vol_my-project_pv1")

	// Should call the remote server
	equals(t, 1, httpmock.GetTotalCallCount())
}
