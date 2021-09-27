package gluster

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/SchweizerischeBundesbahnen/ssp-backend/glusterapi/models"
)

func growVolume(pvName string, newSize string) error {
	if len(pvName) == 0 || len(newSize) == 0 {
		return errors.New("Not all input values provided")
	}

	if err := validateSizeInput(newSize); err != nil {
		return err
	}

	if err := growLvOnAllServers(pvName, newSize); err != nil {
		return err
	}

	return nil
}

func growLvOnAllServers(pvName string, newSize string) error {
	// Create the lv on all other gluster servers
	if err := growLvOnOtherServers(pvName, newSize); err != nil {
		return err
	}

	// Create the lv locally
	if err := growLvLocally(pvName, newSize); err != nil {
		return err
	}

	return nil
}

func growLvOnOtherServers(pvName string, newSize string) error {
	remotes, err := getGlusterPeerServers()
	if err != nil {
		return err
	}

	// Execute the commands remote via API
	client := &http.Client{}
	for _, r := range remotes {
		p := models.GrowVolumeCommand{
			PvName:  pvName,
			NewSize: newSize,
		}
		b := new(bytes.Buffer)

		if err = json.NewEncoder(b).Encode(p); err != nil {
			log.Println("Error encoding json", err.Error())
			return errors.New(commandExecutionError)
		}

		log.Println("Going to grow lv on remote:", r)

		req, _ := http.NewRequest("POST", fmt.Sprintf("http://%v:%v/sec/lv/grow", r, Port), b)
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

func growLvLocally(pvName string, newSize string) error {
	lvName := fmt.Sprintf("lv_%v", pvName)

	commands := []string{
		// Grow lv
		fmt.Sprintf("lvextend -L %v /dev/%v/%v", newSize, VgName, lvName),

		// Grow file system
		fmt.Sprintf("xfs_growfs /dev/%v/%v", VgName, lvName),
	}

	if err := executeCommandsLocally(commands); err != nil {
		return err
	}

	return nil
}
