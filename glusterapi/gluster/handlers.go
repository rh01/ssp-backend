package gluster

import (
	"log"
	"net/http"

	"github.com/SchweizerischeBundesbahnen/ssp-backend/glusterapi/models"
	"github.com/gin-gonic/gin"
)

const (
	wrongAPIUsageError = "Wrong API usage. Your payload did not match the endpoint"
)

func CreateVolumeHandler(c *gin.Context) {
	var json models.CreateVolumeCommand
	if c.BindJSON(&json) == nil {
		log.Printf("Got new request for a volume. project: %v size: %v", json.Project, json.Size)

		if pvName, err := createVolume(json.Project, json.Size); err != nil {
			log.Print("Volume creation failed", err.Error())

			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
		} else {
			log.Print("Volume was created. Name of PV:", pvName)

			c.JSON(http.StatusOK, gin.H{
				"message": pvName,
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"message": wrongAPIUsageError})
	}
}

func CreateLVHandler(c *gin.Context) {
	var json models.CreateLVCommand
	if c.BindJSON(&json) == nil {
		log.Printf("Got new request for a lv. lvName: %v size: %v mountPoint: %v", json.LvName, json.Size, json.MountPoint)

		if err := createLvOnPool(json.Size, json.MountPoint, json.LvName); err != nil {
			log.Print("LV creation failed", err.Error())

			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
		} else {
			log.Print("LV was created")

			c.JSON(http.StatusOK, gin.H{
				"message": "LV created",
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"message": wrongAPIUsageError})
	}
}

func GrowVolumeHandler(c *gin.Context) {
	var json models.GrowVolumeCommand
	if c.BindJSON(&json) == nil {
		log.Printf("Got new request to grow volume. PvName: %v, NewSize: %v", json.PvName, json.NewSize)

		if err := growVolume(json.PvName, json.NewSize); err != nil {
			log.Println("Growing volume failed", err.Error())

			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
		} else {
			log.Print("Volume size successfully increased")

			c.JSON(http.StatusOK, gin.H{
				"message": "Volume was resized",
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"message": wrongAPIUsageError})
	}
}

func GrowLVHandler(c *gin.Context) {
	var json models.GrowVolumeCommand
	if c.BindJSON(&json) == nil {
		log.Printf("Got new request to grow LV. PvName: %v, NewSize: %v", json.PvName, json.NewSize)

		if err := growLvLocally(json.PvName, json.NewSize); err != nil {
			log.Print("Growing LV failed", err.Error())

			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
		} else {
			log.Print("LV was grown")

			c.JSON(http.StatusOK, gin.H{
				"message": "LV was grown",
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"message": wrongAPIUsageError})
	}
}

func VolumeInfoHandler(c *gin.Context) {
	pvName := c.Param("pvname")
	if len(pvName) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": wrongAPIUsageError})
		return
	}

	volInfo, err := getVolumeUsage(pvName)
	if err != nil {
		log.Print("Error getting volume information", err.Error())

		c.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
		})
	} else {
		c.JSON(http.StatusOK, volInfo)
	}
}

func CheckVolumeHandler(c *gin.Context) {
	pvName := c.Param("pvname")
	threshold := c.Query("threshold")
	if len(pvName) == 0 || len(threshold) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": wrongAPIUsageError})
		return
	}

	err := checkVolumeUsage(pvName, threshold)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"message": "Usage is below threshold",
		})
	}
}

func DeleteVolumeHandler(c *gin.Context) {
	var json models.DeleteVolumeCommand
	if c.BindJSON(&json) == nil {
		log.Printf("Got new request to delete LV. lvName: %v", json.LvName)

		if err := deleteVolume(json.LvName); err != nil {
			log.Print("Deleting LV failed", err.Error())

			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
		} else {
			log.Print("LV was deleted")

			c.JSON(http.StatusOK, gin.H{
				"message": "LV was deleted",
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"message": wrongAPIUsageError})
	}
}

func DeleteLVHandler(c *gin.Context) {
	var json models.DeleteVolumeCommand
	if c.BindJSON(&json) == nil {
		log.Printf("Got new request to delete LV. lvName: %v", json.LvName)

		if err := deleteLvLocally(json.LvName); err != nil {
			log.Print("Deleting LV failed", err.Error())

			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
		} else {
			log.Print("LV was deleted")

			c.JSON(http.StatusOK, gin.H{
				"message": "LV was deleted",
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"message": wrongAPIUsageError})
	}
}
