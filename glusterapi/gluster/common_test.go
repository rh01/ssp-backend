package gluster

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"runtime"
)

type TestRunner struct{}

var commands []string
var output []string

func (r TestRunner) Run(command string, args ...string) ([]byte, error) {
	commands = append(commands, command+" "+strings.Join(args, " "))

	// Shift first command out
	var current string
	if len(output) > 0 {
		current, output = output[0], output[1:]
	} else {
		current = ""
	}

	return []byte(current), nil
}

func assert(tb testing.TB, condition bool, msg string, v ...interface{}) {
	if !condition {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: "+msg+"\033[39m\n\n", append([]interface{}{filepath.Base(file), line}, v...)...)
		tb.FailNow()
	}
}

func ok(tb testing.TB, err error) {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: unexpected error: %s\033[39m\n\n", filepath.Base(file), line, err.Error())
		tb.FailNow()
	}
}

func equals(tb testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
		tb.FailNow()
	}
}

func init() {
	ExecRunner = TestRunner{}
}

func isTravis() bool {
	travis := os.Getenv("TRAVIS")
	if len(travis) == 0 {
		travis = "false"
	}
	travisBool, err := strconv.ParseBool(travis)
	if err != nil {
		fmt.Printf("Could not parse TRAVIS environment variable")
		return false
	}
	return travisBool
}

// Test the common functions
func TestGetGlusterPeerServers(t *testing.T) {
	ip1 := "192.168.125.236"
	ip2 := "192.168.125.238"

	output = []string{fmt.Sprintf(`Hostname: %v
						  Hostname: %v`, ip1, ip2)}

	servers, _ := getGlusterPeerServers()

	equals(t, servers[0], ip1)
	equals(t, servers[1], ip2)
}

func TestGetLocalServersIP(t *testing.T) {
	localIP, _ := getLocalServersIP()

	// Make sure response is a valid ip
	ip := net.ParseIP(localIP)

	// Fails on travis
	if !isTravis() {
		assert(t, ip.To4() != nil, "Expected to get local ip, but got "+localIP)
	}
}
