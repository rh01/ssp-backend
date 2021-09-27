package common

import (
	"crypto/rand"
	"fmt"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/keycloak"
	"github.com/gin-gonic/gin"
	"log"
	"strings"
)

// GetUserName returns the username based of the gin.Context
func GetUserName(c *gin.Context) string {
	return keycloak.GetUserName(c)
}

// GetUserMail returns the users mail address based of the gin.Context
func GetUserMail(c *gin.Context) string {
	return keycloak.GetEmail(c)
}

func RandomString(length int) string {
	key := make([]byte, length)
	_, err := rand.Read(key)
	if err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%x", key)
}

func ContainsEmptyString(ss ...string) bool {
	for _, s := range ss {
		if s == "" {
			return true
		}
	}
	return false
}

func ContainsStringI(s []string, e string) bool {
	for _, a := range s {
		if strings.ToLower(a) == strings.ToLower(e) {
			return true
		}
	}
	return false
}

func RemoveDuplicates(elements []string) []string {
	encountered := map[string]bool{}

	// Create a map of all unique elements.
	for v := range elements {
		encountered[elements[v]] = true
	}

	// Place all keys from the map into a slice.
	result := []string{}
	for key := range encountered {
		result = append(result, key)
	}
	return result
}
