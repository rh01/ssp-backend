package gluster

import (
	"testing"

	"github.com/jarcoal/httpmock"
)

func init() {
	ExecRunner = TestRunner{}
}

func TestGetNextVolumeNrForProject_FirstVolume(t *testing.T) {
	output = []string{
		"lv_another_pv1",
	}
	nr, _ := getNextVolumeNrForProject("myproject")
	assert(t, nr == 1, "First volume should return lv number 1")
}

func TestGetNextVolumeNrForProject_SecondVolume(t *testing.T) {
	output = []string{
		"lv_myproject_pv1",
	}
	nr, _ := getNextVolumeNrForProject("myproject")
	assert(t, nr == 2, "First volume should return lv number 2")
}

func TestGetNextVolumeNrForProject_AfterDeletion(t *testing.T) {
	output = []string{
		`lv_myproject_pv1
		lv_myproject_pv27
		lv_myproject_pv3`,
	}
	nr, _ := getNextVolumeNrForProject("myproject")
	equals(t, 28, nr)
}

func TestCreateVolume_Empty(t *testing.T) {
	_, err := createVolume("", "")
	assert(t, err != nil, "createVolume should throw error if called empty")
}

func TestCreateVolume_WrongSize(t *testing.T) {
	_, err := createVolume("pv", "101G")
	assert(t, err != nil, "createVolume should throw error if called with wrong size")
}

func TestCreateVolume_WrongSizeMB(t *testing.T) {
	_, err := createVolume("pv", "1025M")
	assert(t, err != nil, "createVolume should throw error if called with wrong size")
}

func TestCreateVolume(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("POST", "http://192.168.125.236:0/sec/lv",
		httpmock.NewStringResponder(200, ""))

	commands = nil
	output = []string{
		"lvs",
		"Hostname: 192.168.125.236",
	}
	VgName = "vgname"
	BasePath = "/basepath"
	Replicas = 2

	createVolume("my-project", "10M")

	// Should call the remote server
	equals(t, 1, httpmock.GetTotalCallCount())

	// Should execute commands locally
	equals(t, "bash -c mkdir -p /basepath/my-project/pv1", commands[2])
	equals(t, "bash -c lvcreate -V 10M -T vgname/ -n lv_my-project_pv1", commands[3])
	equals(t, "bash -c mkfs.xfs -i size=512 -n size=8192 /dev/vgname/lv_my-project_pv1", commands[4])
	equals(t, "bash -c echo \"/dev/vgname/lv_my-project_pv1 /basepath/my-project/pv1 xfs rw,inode64,noatime,nouuid 1 2\" | tee -a /etc/fstab > /dev/null ", commands[5])
	equals(t, "bash -c mount -o rw,inode64,noatime,nouuid /dev/vgname/lv_my-project_pv1 /basepath/my-project/pv1", commands[6])
	equals(t, "bash -c mkdir /basepath/my-project/pv1/brick", commands[7])
	equals(t, "bash -c semanage fcontext -a -t glusterd_brick_t /basepath/my-project/pv1/brick", commands[8])
	equals(t, "bash -c restorecon -Rv /basepath/my-project/pv1/brick", commands[9])
	equals(t, "bash -c chown nfsnobody.nfsnobody /basepath/my-project/pv1/brick", commands[10])
	equals(t, "bash -c chmod 777 /basepath/my-project/pv1/brick", commands[11])

	if !isTravis() {
		ip, _ := getLocalServersIP()
		equals(t, "bash -c gluster volume create vol_my-project_pv1 replica 2 "+ip+":/basepath/my-project/pv1/brick ", commands[13])
		equals(t, "bash -c gluster volume start vol_my-project_pv1", commands[14])
	}
}
