package gluster

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/SchweizerischeBundesbahnen/ssp-backend/glusterapi/models"
)

func getVolumeUsage(pvName string) (*models.VolInfo, error) {
	// Input examples
	// gl-ose-mon-a-pv3 => ose--mon--a_pv3
	// gl-test-pv10 => test_pv10

	// LVS example output
	// /dev/mapper/vg_slow-lv_test_pv5
	// /dev/mapper/vg_san-lv_ose--mon--a_pv2

	// Remove gl_ if necessary
	pvName = strings.Replace(pvName, "gl-", "", -1)

	// Find project name
	parts := strings.Split(pvName, "-pv")

	// Replace - with -- within project name
	cmd := fmt.Sprintf("df --output=size,used,source | grep 'lv_%v_pv%v$'", strings.Replace(parts[0], "-", "--", -1), parts[1])

	out, err := ExecRunner.Run("bash", "-c", cmd)
	if err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			return nil, fmt.Errorf("PV %v does not exist", pvName)
		}
		msg := "Could not parse usage size: " + err.Error()
		log.Println(msg)
		return nil, errors.New(msg)
	}

	volInfo, err := parseOutput(string(out))
	if err != nil {
		return nil, err
	}
	return volInfo, nil
}

func parseOutput(stdOut string) (*models.VolInfo, error) {
	// Example output
	// 5472   118048 /dev/mapper/vg_slow-lv_test_pv5
	num := regexp.MustCompile("(\\d+)")
	nums := num.FindAllString(stdOut, -1)

	size, err := strconv.Atoi(nums[0])
	if err != nil {
		log.Println("Unable to parse size value of df output", stdOut)
		return nil, errors.New(commandExecutionError)
	}

	used, err := strconv.Atoi(nums[1])
	if err != nil {
		log.Println("Unable to parse used value of df output", stdOut)
		return nil, errors.New(commandExecutionError)
	}

	return &models.VolInfo{
		TotalKiloBytes: size,
		UsedKiloBytes:  used,
	}, nil
}

func checkVolumeUsage(pvName string, threshold string) error {
	t, err := strconv.ParseFloat(threshold, 64)
	if err != nil {
		return errors.New("Wrong threshold. Is not a valid integer")
	}

	volInfo, err := getVolumeUsage(pvName)
	if err != nil {
		return err
	}

	usedPercentage := 100 / float64(volInfo.TotalKiloBytes) * float64(volInfo.UsedKiloBytes)
	if usedPercentage > t {
		return fmt.Errorf("Error used %v is bigger than threshold: %v", usedPercentage, t)
	}

	return nil
}
