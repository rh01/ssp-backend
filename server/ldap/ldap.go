package ldap

import (
	"crypto/tls"
	"fmt"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/common"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
	log "github.com/sirupsen/logrus"

	"gopkg.in/ldap.v2"
)

type LDAPClient struct {
	Conn         *ldap.Conn
	Host         string
	Port         int
	BindDN       string `mapstructure:"dn"`
	BindPassword string `mapstructure:"password"`
	GroupFilter  string // e.g. "(memberUid=%s)"
	UserFilter   string // e.g. "(uid=%s)"
	Base         string
	Attributes   []string
	ADDomainName string // ActiveDirectory domain name "example.com"

	UseSSL             bool
	InsecureSkipVerify bool
	ServerName         string
	SkipTLS            bool
	ClientCertificates []tls.Certificate
}

func New() (*LDAPClient, error) {
	var ldapclient LDAPClient
	l := config.Config().Sub("ldap")
	if l == nil {
		return nil, fmt.Errorf("LDAP configuration missing. Must set host, base, dn and password!")
	}
	l.SetDefault("Port", 389)
	l.SetDefault("UseSSL", false)
	l.SetDefault("SkipTLS", true)
	l.SetDefault("UserFilter", "(cn=%s)")

	if !(l.IsSet("host") && l.IsSet("base") && l.IsSet("dn") && l.IsSet("password")) {
		return nil, fmt.Errorf("LDAP configuration incomplete. Must set host, base, dn and password!")
	}
	if err := l.Unmarshal(&ldapclient); err != nil {
		return nil, err
	}
	return &ldapclient, nil
}

// Connect connects to the ldap backend
func (lc *LDAPClient) Connect() error {
	if lc.Conn == nil {
		var l *ldap.Conn
		var err error
		address := fmt.Sprintf("%s:%d", lc.Host, lc.Port)
		if !lc.UseSSL {
			l, err = ldap.Dial("tcp", address)
			if err != nil {
				return err
			}

			// Reconnect with TLS
			if !lc.SkipTLS {
				err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
				if err != nil {
					return err
				}
			}
		} else {
			config := &tls.Config{
				InsecureSkipVerify: lc.InsecureSkipVerify,
				ServerName:         lc.ServerName,
			}

			if lc.ClientCertificates != nil && len(lc.ClientCertificates) > 0 {
				config.Certificates = lc.ClientCertificates
			}

			l, err = ldap.DialTLS("tcp", address, config)
			if err != nil {
				return err
			}
		}

		lc.Conn = l
	}
	return nil
}

// Close closes the ldap backend connection
func (lc *LDAPClient) Close() {
	if lc.Conn != nil {
		lc.Conn.Close()
		lc.Conn = nil
	}
}

func getGroupBlacklist() []string {
	cfg := config.Config()
	var blacklist []string
	// We can ignore the error here
	if err := cfg.UnmarshalKey("ldap.group_blacklist", &blacklist); err != nil {
		log.Warn("No LDAP group blacklist found")
	}
	return blacklist
}

func (lc *LDAPClient) GetUser(username string) (*ldap.Entry, error) {
	err := lc.Connect()
	if err != nil {
		return nil, err
	}

	// First bind with a read only user
	if lc.BindDN != "" && lc.BindPassword != "" {
		err = lc.Conn.Bind(lc.BindDN, lc.BindPassword)
		if err != nil {
			return nil, err
		}
	}

	searchRequest := ldap.NewSearchRequest(
		lc.Base,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(lc.UserFilter, username),
		[]string{"memberOf"},
		nil,
	)
	sr, err := lc.Conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}
	if len(sr.Entries) > 1 {
		return nil, fmt.Errorf("Something went wrong. Multiple LDAP users returned")
	}
	return sr.Entries[0], nil
}

func getCN(dn string) string {
	parsedDN, err := ldap.ParseDN(dn)
	fields := log.Fields{"dn": dn}
	if err != nil {
		log.WithFields(fields).Error("Could not parse DN")
		return ""
	}
	// Die erste RDN ist immer CN, dieser hat nur ein Attribut
	if len(parsedDN.RDNs) < 1 {
		//log.WithFields(fields).Error("Unexpected RDNs length")
		return ""
	}
	if len(parsedDN.RDNs[0].Attributes) != 1 {
		log.WithFields(fields).Error("Unexpected attributes length")
		return ""
	}
	return parsedDN.RDNs[0].Attributes[0].Value
}

func (lc *LDAPClient) GetGroupsOfUser(username string) ([]string, error) {
	var groups []string
	user, err := lc.GetUser(username)
	if err != nil {
		return groups, err
	}
	blacklist := getGroupBlacklist()
	for _, entry := range user.GetAttributeValues("memberOf") {
		group := getCN(entry)
		// Check if the group is blacklisted
		if common.ContainsStringI(blacklist, group) {
			continue
		}
		groups = append(groups, group)
	}
	return groups, nil
}
