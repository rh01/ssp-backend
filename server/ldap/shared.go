package ldap

import (
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/common"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

const (
	genericAPIError = "An Error has occured while getting your LDAP groups. Please create an Issue."
)

func RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/ldap/groups", listLdapGroupsHandler)
}

func listLdapGroupsHandler(c *gin.Context) {
	username := common.GetUserName(c)
	l, err := New()
	if err != nil {
		log.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAPIError})
		return
	}
	defer l.Close()

	groups, err := l.GetGroupsOfUser(username)
	if err != nil {
		log.Errorf("%v", err)
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: genericAPIError})
		return
	}
	log.WithFields(log.Fields{
		"groups":   groups,
		"username": username,
	}).Debug("LDAP groups")

	c.JSON(http.StatusOK, groups)
}
