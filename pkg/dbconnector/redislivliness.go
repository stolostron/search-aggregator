/*
IBM Confidential
OCO Source Materials
(C) Copyright IBM Corporation 2019 All Rights Reserved
The source code for this program is not published or otherwise divested of its trade secrets, irrespective of what has been deposited with the U.S. Copyright Office.
*/

package dbconnector

import (
	"time"

	"github.com/golang/glog"
	"github.com/open-cluster-management/search-aggregator/pkg/config"
)

var existingClustersMap map[string]map[string]interface{} // a map to hold Current properties already pushed to RedisGraph using SET

func redisWatcher() {
	conn := Pool.Get()
	interval := time.Duration(config.Cfg.RedisWatchRate) * time.Millisecond

	for {
		_, err := conn.Do("PING")
		if err != nil {
			glog.Warningf("Failed to PING redis - clear in memory data ")
			clearClusterCache()
			conn.Close()
			break
		}
		time.Sleep(interval)
	}

}

func clearClusterCache() {
	existingClustersMap = nil
	ExistingIndexMap = nil
}

func createClustersCache(key string, val map[string]interface{}) {
	if existingClustersMap != nil { // this should not happen
		glog.V(4).Infof("Trying to start duplicate RedisWatcher")
		return
	} else {
		existingClustersMap = make(map[string]map[string]interface{})
		existingClustersMap[key] = val
		//  if Redis is up start Watcher
		conn := Pool.Get()
		_, err := conn.Do("PING")
		conn.Close()
		if err != nil {
			clearClusterCache()
		} else {
			go redisWatcher()
		}

	}

}

func setClustersCache(key string, val map[string]interface{}) {
	existingClustersMap[key] = val
}

func isClustersCacheNil() bool {
	return existingClustersMap == nil
}
func getClustersCache(key string) map[string]interface{} {
	return existingClustersMap[key]
}

func isKeyClustersCache(key string) bool {
	if existingClustersMap == nil {
		return false
	}
	_, ok := existingClustersMap[key]
	return ok
}

func DeleteClustersCache(key string) {
	_, ok := existingClustersMap[key]
	if ok {
		delete(existingClustersMap, key)
	}
}
