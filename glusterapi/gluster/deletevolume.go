package gluster

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/SchweizerischeBundesbahnen/ssp-backend/glusterapi/models"
)

func deleteVolume(volName string) error {
	if len(volName) == 0 {
		return errors.New("Not all input values provided")
	}

	if err := deleteGlusterVolume(volName); err != nil {
		return err
	}

	if err := deleteLvOnAllServers(volName); err != nil {
		return err
	}

	return nil
}

func deleteLvOnAllServers(lvName string) error {
	// Delete the lv on all other gluster servers
	if err := deleteLvOnOtherServers(lvName); err != nil {
		return err
	}

	// Delete the lv locally
	if err := deleteLvLocally(lvName); err != nil {
		return err
	}

	return nil
}

func deleteLvOnOtherServers(lvName string) error {
	remotes, err := getGlusterPeerServers()
	if err != nil {
		return err
	}

	// Execute the commands remote via API
	client := &http.Client{}
	for _, r := range remotes {
		p := models.DeleteVolumeCommand{
			LvName: lvName,
		}
		b := new(bytes.Buffer)

		if err = json.NewEncoder(b).Encode(p); err != nil {
			log.Println("Error encoding json", err.Error())
			return errors.New(commandExecutionError)
		}

		log.Println("Going to delete lv on remote:", r)

		req, _ := http.NewRequest("POST", fmt.Sprintf("http://%v:%v/sec/lv/delete", r, Port), b)
		req.SetBasicAuth("GLUSTER_API", Secret)

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				log.Println("Remote did not respond with OK", resp.StatusCode)
			} else {
				log.Println("Connection to remote not possible", r, err.Error())
			}
			return errors.New(commandExecutionError)
		}
		resp.Body.Close()
	}
	return nil
}

func deleteGlusterVolume(volName string) error {
	commands := []string{
		fmt.Sprintf("gluster volume stop %v --mode=script", volName),
		fmt.Sprintf("gluster volume delete %v --mode=script", volName),
	}

	if err := executeCommandsLocally(commands); err != nil {
		return err
	}

	return nil
}

func getMountPath(volName string) string {
	basePath := strings.Replace(volName, "vol_", BasePath+"/", 1)
	return strings.Replace(basePath, "_pv", "/pv", 1)
}

func deleteLvLocally(volName string) error {
	mountPath := getMountPath(volName)
	lvName := strings.Replace(volName, "vol_", "lv_", 1)
	commands := []string{
		fmt.Sprintf("sed -i '\\#/dev/%v/%v#d' /etc/fstab", VgName, lvName),
		fmt.Sprintf("umount %v", string(mountPath)),
		fmt.Sprintf("lvremove --yes /dev/%v/%v", VgName, lvName),
		// delete all empty parents. commands fails on first non-empty parent, so ignore failure
		fmt.Sprintf("rmdir --parents --ignore-fail-on-non-empty %v", string(mountPath)),
	}

	if err := executeCommandsLocally(commands); err != nil {
		return err
	}

	return nil
}
