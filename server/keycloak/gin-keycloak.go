package keycloak

import (
	"crypto/rsa"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/SchweizerischeBundesbahnen/ssp-backend/server/config"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"gopkg.in/square/go-jose.v2/jwt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// totally stolen from https://github.com/tbaehler/gin-keycloak

// VarianceTimer controls the max runtime of Auth() and AuthChain() middleware
var VarianceTimer time.Duration = 30000 * time.Millisecond
var Transport = http.Transport{}
var publicKeyCache = cache.New(8*time.Hour, 8*time.Hour)

// TokenContainer stores all relevant token information
type TokenContainer struct {
	Token         *oauth2.Token
	KeyCloakToken *KeyCloakToken
}

// AccessCheckFunction is a function that checks if a given token grants
// access.
type AccessCheckFunction func(tc *TokenContainer, ctx *gin.Context) bool

type KeyCloakToken struct {
	Jti               string                 `json:"jti"`
	Exp               int64                  `json:"exp"`
	Nbf               int64                  `json:"nbf"`
	Iat               int64                  `json:"iat"`
	Iss               string                 `json:"iss"`
	Sub               string                 `json:"sub"`
	Typ               string                 `json:"typ"`
	Azp               string                 `json:"azp"`
	Nonce             string                 `json:"nonce"`
	AuthTime          int64                  `json:"auth_time"`
	SessionState      string                 `json:"session_state"`
	Acr               string                 `json:"acr"`
	ClientSession     string                 `json:"client_session"`
	AllowedOrigins    []string               `json:"allowed-origins"`
	ResourceAccess    map[string]ServiceRole `json:"resource_access"`
	Name              string                 `json:"name"`
	PreferredUsername string                 `json:"preferred_username"`
	UID               string                 `json:"sbbuid_ad"`
	GivenName         string                 `json:"given_name"`
	FamilyName        string                 `json:"family_name"`
	Email             string                 `json:"email"`
}

type ServiceRole struct {
	Roles []string `json:"roles"`
}

func extractToken(r *http.Request) (*oauth2.Token, error) {
	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		return nil, errors.New("No authorization header")
	}

	th := strings.Split(hdr, " ")
	if len(th) != 2 {
		return nil, errors.New("Incomplete authorization header")
	}

	return &oauth2.Token{AccessToken: th[1], TokenType: th[0]}, nil
}

func GetUserName(ctx *gin.Context) string {
	tokenContainer, ok := getTokenContainer(ctx)
	if !ok {
		return ""
	} else {
		//uid := strings.Split(tokenContainer.KeyCloakToken.PreferredUsername, "\\")[1]
		uid := tokenContainer.KeyCloakToken.UID
		if len(uid) == 0 {
			return ""
		}
		return uid
	}
}

func GetEmail(ctx *gin.Context) string {
	tokenContainer, ok := getTokenContainer(ctx)
	if !ok {
		return ""
	} else {
		return tokenContainer.KeyCloakToken.Email
	}
}

func GetTokenContainer(token *oauth2.Token) (*TokenContainer, error) {

	keyCloakToken, err := decodeToken(token)
	if err != nil {
		return nil, err
	}

	return &TokenContainer{
		Token: &oauth2.Token{
			AccessToken: token.AccessToken,
			TokenType:   token.TokenType,
		},
		KeyCloakToken: keyCloakToken,
	}, nil
}

func getPublicKey(keyId string) (string, string, error) {
	keyEntry, exists := publicKeyCache.Get(keyId)
	if !exists {

		ssoURL := config.Config().GetString("sso_url")
		ssoRealm := config.Config().GetString("sso_realm")
		if ssoURL == "" || ssoRealm == "" {
			return "", "", errors.New("Missing SSO configuration")
		}
		ssoCertsURL := ssoURL + "/realms/" + ssoRealm + "/protocol/openid-connect/certs"

		// Create http client with proxy:
		// https://blog.abhi.host/blog/2016/02/27/golang-creating-https-connection-via/
		// somehow doesn't work with default environment variable (?)
		client := &http.Client{}
		httpProxy := os.Getenv("http_proxy")
		if httpProxy == "" {
			httpProxy = os.Getenv("HTTP_PROXY")
		}
		if httpProxy != "" {
			proxyURL, err := url.Parse(httpProxy)
			if err != nil {
				log.Printf(err.Error())
			}

			transport := http.Transport{
				Proxy:           http.ProxyURL(proxyURL),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			client.Transport = &transport
		}

		req, err := http.NewRequest("GET", ssoCertsURL, nil)
		if err != nil {
			log.Printf(err.Error())
			return "", "", err
		}

		log.Debugf("Calling %v", req.URL.String())

		resp, err := client.Do(req)
		if err != nil {
			log.Println("Error from server: ", err.Error())
			return "", "", err
		}
		defer resp.Body.Close()

		// TODO: check statuscode

		body, err := ioutil.ReadAll(resp.Body)

		var data map[string][]map[string]string
		if err := json.Unmarshal(body, &data); err != nil {
			return "", "", err
		}
		keyEntry = data["keys"]
		publicKeyCache.Set(keyId, keyEntry, cache.DefaultExpiration)
	}

	for _, keyIdFromServer := range keyEntry.([]map[string]string) {
		if keyIdFromServer["kid"] == keyId {
			return keyIdFromServer["n"], keyIdFromServer["e"], nil
		}

	}

	return "", "", errors.New("no key found")
}

func decodeToken(token *oauth2.Token) (*KeyCloakToken, error) {
	keyCloakToken := KeyCloakToken{}
	var err error
	parsedJWT, err := jwt.ParseSigned(token.AccessToken)
	if err != nil {
		log.Errorf("[Gin-OAuth] jwt not decodable: %s", err)
		return nil, err
	}
	n, e, err := getPublicKey(parsedJWT.Headers[0].KeyID)
	if err != nil {
		log.Errorf("Failed to get publickey %v", err)
		return nil, err
	}
	num, _ := base64.RawURLEncoding.DecodeString(n)

	bigN := new(big.Int)
	bigN.SetBytes(num)
	num, _ = base64.RawURLEncoding.DecodeString(e)
	bigE := new(big.Int)
	bigE.SetBytes(num)
	key := rsa.PublicKey{bigN, int(bigE.Int64())}

	err = parsedJWT.Claims(&key, &keyCloakToken)
	if err != nil {
		log.Errorf("Failed to get claims JWT:%+v", err)
		return nil, err
	}
	return &keyCloakToken, nil
}

func isExpired(token *KeyCloakToken) bool {
	if token.Exp == 0 {
		return false
	}
	now := time.Now()
	fromUnixTimestamp := time.Unix(token.Exp, 0)
	return now.After(fromUnixTimestamp)
}

func getTokenContainer(ctx *gin.Context) (*TokenContainer, bool) {
	var oauthToken *oauth2.Token
	var tc *TokenContainer
	var err error

	if oauthToken, err = extractToken(ctx.Request); err != nil {
		log.Errorf("[Gin-OAuth] Can not extract oauth2.Token, caused by: %s", err)
		return nil, false
	}
	if !oauthToken.Valid() {
		log.Infof("[Gin-OAuth] Invalid Token - nil or expired")
		return nil, false
	}

	if tc, err = GetTokenContainer(oauthToken); err != nil {
		log.Errorf("[Gin-OAuth] Can not extract TokenContainer, caused by: %s", err)
		return nil, false
	}

	if isExpired(tc.KeyCloakToken) {
		log.Errorf("[Gin-OAuth] Keycloak Token has expired")
		return nil, false
	}

	return tc, true
}

func (t *TokenContainer) Valid() bool {
	if t.Token == nil {
		return false
	}
	return t.Token.Valid()
}

func Auth(accessCheckFunction AccessCheckFunction) gin.HandlerFunc {
	return AuthChain(accessCheckFunction)
}

func AuthChain(accessCheckFunctions ...AccessCheckFunction) gin.HandlerFunc {
	// middleware
	return func(ctx *gin.Context) {
		t := time.Now()
		varianceControl := make(chan bool, 1)

		go func() {
			tokenContainer, ok := getTokenContainer(ctx)
			if !ok {
				ctx.AbortWithError(http.StatusUnauthorized, errors.New("No token in context"))
				varianceControl <- false
				return
			}

			if !tokenContainer.Valid() {
				ctx.AbortWithError(http.StatusUnauthorized, errors.New("Invalid Token"))
				varianceControl <- false
				return
			}

			for i, fn := range accessCheckFunctions {
				if fn(tokenContainer, ctx) {
					varianceControl <- true
					break
				}

				if len(accessCheckFunctions)-1 == i {
					ctx.AbortWithError(http.StatusForbidden, errors.New("Access to the Resource is fobidden"))
					varianceControl <- false
					return
				}
			}
		}()

		select {
		case ok := <-varianceControl:
			if !ok {
				log.Debugf("[Gin-OAuth] %12v %s access not allowed", time.Since(t), ctx.Request.URL.Path)
				return
			}
		case <-time.After(VarianceTimer):
			ctx.AbortWithError(http.StatusGatewayTimeout, errors.New("Authorization check overtime"))
			log.Debugf("[Gin-OAuth] %12v %s overtime", time.Since(t), ctx.Request.URL.Path)
			return
		}

		log.Debugf("[Gin-OAuth] %12v %s access allowed", time.Since(t), ctx.Request.URL.Path)
	}
}

func RequestLogger(keys []string, contentKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		request := c.Request
		c.Next()
		err := c.Errors
		if request.Method != "GET" && err == nil {
			data, e := c.Get(contentKey)
			if e != false { //key is non existent
				values := make([]string, 0)
				for _, key := range keys {
					val, keyPresent := c.Get(key)
					if keyPresent {
						values = append(values, val.(string))
					}
				}
				log.Infof("[Gin-OAuth] Request: %+v for %s", data, strings.Join(values, "-"))
			}
		}
	}
}
