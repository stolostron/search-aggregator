/*
IBM Confidential
OCO Source Materials
5737-E67
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/golang/glog"
)

const (
	DEFAULT_AGGREGATOR_ADDRESS   = ":3010"
	DEFAULT_REDIS_HOST           = "localhost"
	DEFAULT_REDIS_PORT           = "6379"
	DEFAULT_REDISCOVER_RATE_MS   = 60000  // 1 min
	DEFAULT_HTTP_TIMEOUT         = 300000 // 5 min, to fix the EOF response at the collector
	DEFAULT_EDGE_BUILD_RATE_MS   = 10000  // 10 sec
	AGGREGATOR_API_VERSION       = "3.2.1"
	DEFAULT_REDIS_WATCH_INTERVAL = 15000 //15 seconds
)

// Define a config type to hold our config properties.
type Config struct {
	AggregatorAddress string // address for collector <-> aggregator
	RedisHost         string // host path for redis
	RedisPort         string // port for redis
	RedisSSHPort      string // ssh port for redis
	RedisPassword     string // password for redis
	KubeConfig        string // Local kubeconfig path
	RediscoverRateMS  int    // time in MS we should check on cluster resource type
	EdgeBuildRateMS   int    // rate at which intercluster edges should be build
	RedisWatchRate    int    // rate at which Redis Ping hapens to check health
	HTTPTimeout       int    // timeout when the http server should drop connections
}

var Cfg = Config{}

func init() {
	// parse flags
	flag.Parse()
	err := flag.Lookup("logtostderr").Value.Set("true") // Glog is weird in that by default it logs to a file. Change it so that by default it all goes to stderr. (no option for stdout).
	if err != nil {
		fmt.Println("Error setting default flag:", err) // Uses fmt.Println in case something is wrong with glog args
		os.Exit(1)
		glog.Fatal("Error setting default flag: ", err)
	}
	defer glog.Flush() // This should ensure that everything makes it out on to the console if the program crashes.

	// If environment variables are set, use those values constants
	// Simply put, the order of preference is env -> default constants (from left to right)
	setDefault(&Cfg.AggregatorAddress, "AGGREGATOR_ADDRESS", DEFAULT_AGGREGATOR_ADDRESS)
	setDefault(&Cfg.RedisHost, "REDIS_HOST", DEFAULT_REDIS_HOST)
	setDefault(&Cfg.RedisPort, "REDIS_PORT", DEFAULT_REDIS_PORT)
	setDefault(&Cfg.RedisSSHPort, "REDIS_SSH_PORT", "")
	setDefault(&Cfg.RedisPassword, "REDIS_PASSWORD", "")
	setDefaultInt(&Cfg.RediscoverRateMS, "REDISCOVER_RATE_MS", DEFAULT_REDISCOVER_RATE_MS)
	setDefaultInt(&Cfg.EdgeBuildRateMS, "EDGE_BUILD_RATE_MS", DEFAULT_EDGE_BUILD_RATE_MS)
	setDefaultInt(&Cfg.RedisWatchRate, "REDIS_WATCH_RATE_MS", DEFAULT_REDIS_WATCH_INTERVAL)
	setDefaultInt(&Cfg.HTTPTimeout, "HTTP_TIMEOUT", DEFAULT_HTTP_TIMEOUT)

	defaultKubePath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if _, err := os.Stat(defaultKubePath); os.IsNotExist(err) {
		// set default to empty string if path does not reslove
		defaultKubePath = ""
	}
	setDefault(&Cfg.KubeConfig, "KUBECONFIG", defaultKubePath)
}

func setDefault(field *string, env, defaultVal string) {
	if val := os.Getenv(env); val != "" {
		if env == "REDIS_PASSWORD" {
			glog.Infof("Using %s from environment", env)
		} else {
			glog.Infof("Using %s from environment: %s", env, val)
		}
		*field = val
	} else if *field == "" && defaultVal != "" {
		glog.Infof("%s not set, using default value: %s", env, defaultVal)
		*field = defaultVal
	}
}

// TODO: Combine with function above.
func setDefaultInt(field *int, env string, defaultVal int) {
	if val := os.Getenv(env); val != "" {
		glog.Infof("Using %s from environment: %s", env, val)
		var err error
		*field, err = strconv.Atoi(val)
		if err != nil {
			glog.Error("Error parsing env [", env, "].  Expected an integer.  Original error: ", err)
		}
	} else if *field == 0 && defaultVal != 0 {
		glog.Infof("No %s from file or environment, using default value: %d", env, defaultVal)
		*field = defaultVal
	}
}
