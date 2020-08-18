package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseCacheKey(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(CacheKey{}, parseCacheKey("cat", []string{"test.txt"}))

	assert.Equal(CacheKey{ClusterID: "clusterid"}, parseCacheKey("aws-iam-authenticator", []string{"token", "-i", "clusterid"}))

	assert.Equal(CacheKey{ClusterID: "clusterid", Region: "us-west-2"}, parseCacheKey("aws", []string{"--region", "us-west-2", "eks", "get-token", "--cluster-name", "clusterid"}))
	assert.Equal(CacheKey{ClusterID: "clusterid"}, parseCacheKey("aws", []string{"eks", "get-token", "--cluster-name", "clusterid"}))
	assert.Equal(CacheKey{}, parseCacheKey("aws", []string{"eks", "get-token", "--cluster-name"}))
}

func TestGetCacheFileName(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("clusterid_AKIA", getCacheFileName(getCacheKey("aws-iam-authenticator", []string{"token", "-i", "clusterid"}, "AKIA")))
	assert.Equal("clusterid_AKIA_us-west-2", getCacheFileName(getCacheKey("aws", []string{"--region", "us-west-2", "eks", "get-token", "--cluster-name", "clusterid"}, "AKIA")))
}

func TestParseExpiry(t *testing.T) {
	assert := assert.New(t)

	ts := parseExpiry(`{"kind": "ExecCredential", "apiVersion": "client.authentication.k8s.io/v1alpha1", "spec": {}, "status": {"expirationTimestamp": "2020-08-17T18:59:13Z", "token": "k8s-aws-v1.LOTS_OF_STUFF_HERE"}}`)
	assert.Equal(2020, ts.Year())
	assert.Equal(time.Month(8), ts.Month())
	assert.Equal(17, ts.Day())
	assert.Equal(18, ts.Hour())
	assert.Equal(59, ts.Minute())
	assert.Equal(13, ts.Second())
}
