package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh/terminal"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

type Token struct {
	Expire string `json:"expire"`
	Token  string `json:"token"`
}

func main() {
	flag.Usage = printUsage
	method := flag.String("X", "GET", "The request method (GET,POST)")
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	u := getURL(flag.Arg(0))
	token, err := getToken()
	if err != nil {
		token, err = login(u)
		if err != nil {
			log.Fatal(err)
		}
	}
	request(u, *method, token.Token)
	if err != nil {
		log.Fatal(err)
	}
}

func getURL(u string) *url.URL {
	if !strings.HasPrefix(u, "http") {
		u = "http://" + u
	}
	parsed, _ := url.Parse(u)
	return parsed
}

func printUsage() {
	fmt.Printf("Usage: %s [OPTIONS] <url>\n", os.Args[0])
	flag.PrintDefaults()
}

func request(u *url.URL, method string, token string) error {
	client := &http.Client{}
	req, err := http.NewRequest(method, u.String(), nil)
	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func promptUsernamePassword() (string, string) {
	fmt.Print("Enter username: ")
	var username string
	fmt.Scanln(&username)
	fmt.Print("Enter password: ")
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println() // newline
	password := string(bytePassword)
	return username, password
}

func login(u *url.URL) (*Token, error) {
	username, password := promptUsernamePassword()
	values := map[string]string{"Username": username, "Password": password}
	jsonValue, _ := json.Marshal(values)
	loginURL := fmt.Sprintf("%s://%s/login", u.Scheme, u.Host)
	resp, err := http.Post(loginURL, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = ioutil.WriteFile("token.json", body, 0600)
	if err != nil {
		return nil, err
	}
	var token Token
	err = json.Unmarshal(body, &token)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func getToken() (*Token, error) {
	raw, err := ioutil.ReadFile("token.json")
	if err != nil {
		return nil, err
	}
	var token Token
	err = json.Unmarshal(raw, &token)
	if err != nil {
		return nil, err
	}
	expire, _ := time.Parse(time.RFC3339, token.Expire)
	if expire.Before(time.Now()) {
		return nil, errors.New("The token has expired")
	}
	return &token, nil
}
