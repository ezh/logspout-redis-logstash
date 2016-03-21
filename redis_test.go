package redis

import (
	"bytes"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/fsouza/go-dockerclient"
	"github.com/gliderlabs/logspout/router"
	"github.com/jmoiron/jsonq"
	"github.com/stretchr/testify/assert"
)

func TestSplitImage(t *testing.T) {
	assert := assert.New(t)

	image, tag := splitImage("bla")
	assert.Equal("bla", image)
	assert.Equal("", tag)

	image, tag = splitImage("foo:latest")
	assert.Equal("foo", image)
	assert.Equal("latest", tag)

	image, tag = splitImage("foo/bar:latest")
	assert.Equal("foo/bar", image)
	assert.Equal("latest", tag)

	image, tag = splitImage("my.registry.host/some/image:1.3.4")
	assert.Equal("my.registry.host/some/image", image)
	assert.Equal("1.3.4", tag)

	image, tag = splitImage("my.registry.host:443/path/to/image:3.1.4")
	assert.Equal("my.registry.host:443/path/to/image", image)
	assert.Equal("3.1.4", tag)

	image, tag = splitImage("my.registry.host:443/path/to/image")
	assert.Equal("my.registry.host:443/path/to/image", image)
	assert.Equal("", tag)

}

func TestCreateLogstashMessageV1(t *testing.T) {

	assert := assert.New(t)

	m := router.Message{
		Container: &docker.Container{
			ID:   "6feffd9428dc",
			Name: "/my_app",
			Config: &docker.Config{
				Hostname: "container_hostname",
				Image:    "my.registry.host:443/path/to/image:1234",
			},
		},
		Source: "stdout",
		Data:   "hello world",
		Time:   time.Unix(int64(1453818496), 595000000),
	}

	msg, _ := createLogstashMessage(&m, "tst-mesos-slave-001", false, "my-type")
	jq := makeQuery(msg)

	assert.Equal("my-type", getString(jq, "@type"))
	assert.Equal("2016-01-26T14:28:16.595Z", getString(jq, "@timestamp"))
	assert.Equal("container_hostname", getString(jq, "host"))
	assert.Equal("hello world", getString(jq, "message"))
	assert.Equal("my_app", getString(jq, "docker", "name"))
	assert.Equal("6feffd9428dc", getString(jq, "docker", "cid"))
	assert.Equal("my.registry.host:443/path/to/image", getString(jq, "docker", "image"))
	assert.Equal("1234", getString(jq, "docker", "image_tag"))
	assert.Equal("stdout", getString(jq, "docker", "source"))
	assert.Equal("tst-mesos-slave-001", getString(jq, "docker", "docker_host"))

}

func TestCreateLogstashMessageOptionalType(t *testing.T) {

	assert := assert.New(t)

	m := router.Message{
		Container: &docker.Container{
			ID:   "f00ffd9428dc",
			Name: "/my_db",
			Config: &docker.Config{
				Hostname: "container_hostname",
				Image:    "my.registry.host:443/path/to/image:4321",
			},
		},
		Source: "stderr",
		Data:   "cruel world",
		Time:   time.Unix(int64(1453813330), 0),
	}

	msg, _ := createLogstashMessage(&m, "tst-mesos-slave-001", true, "")
	jq := makeQuery(msg)
	log.Printf("Standard message: %s", msg)

	assert.Equal("", getString(jq, "@type"))

}

func TestCreateLogstashMessageWithJsonData(t *testing.T) {

	assert := assert.New(t)

	m := router.Message{
		Container: &docker.Container{
			ID:   "6feffd9428dc",
			Name: "/my_app",
			Config: &docker.Config{
				Hostname: "container_hostname",
				Image:    "my.registry.host:443/path/to/image:1234",
			},
		},
		Source: "stdout",
		Data:   `{"logtype": "applog", "message":"something happened", "level": "DEBUG", "file": "debug.go", "line": 42}`,
		Time:   time.Unix(int64(1453818496), 595000000),
	}

	msg, _ := createLogstashMessage(&m, "tst-mesos-slave-001", false, "my-type")
	jq := makeQuery(msg)

	assert.Equal("something happened", getString(jq, "message"))

	log.Printf("Dynamic message: %s", msg)

}

func TestCreateLogstashMessageWithJsonDataAndNoMessage(t *testing.T) {

	assert := assert.New(t)

	m := router.Message{
		Container: &docker.Container{
			ID:   "6feffd9428dc",
			Name: "/my_app",
			Config: &docker.Config{
				Hostname: "container_hostname",
				Image:    "my.registry.host:443/path/to/image:1234",
			},
		},
		Source: "stdout",
		Data:   `{ "logtype": "applog", "level": "DEBUG", "file": "debug.go", "line": 42}`,
		Time:   time.Unix(int64(1453818496), 595000000),
	}

	msg, _ := createLogstashMessage(&m, "tst-mesos-slave-001", false, "my-type")
	jq := makeQuery(msg)

	assert.Equal("no message", getString(jq, "message"))

	log.Printf("Dynamic message: %s", msg)

}

func TestCreateLogstashMessageWithJsonDataAndNoLogtype(t *testing.T) {

	assert := assert.New(t)

	m := router.Message{
		Container: &docker.Container{
			ID:   "6feffd9428dc",
			Name: "/my_app",
			Config: &docker.Config{
				Hostname: "container_hostname",
				Image:    "my.registry.host:443/path/to/image:1234",
			},
		},
		Source: "stdout",
		Data:   `{ "message":"here i am!", "level": "DEBUG", "file": "debug.go", "line": 42}`,
		Time:   time.Unix(int64(1453818496), 595000000),
	}

	msg, _ := createLogstashMessage(&m, "tst-mesos-slave-001", false, "my-type")
	jq := makeQuery(msg)

	assert.Equal("", getString(jq, "logtype"))

	log.Printf("Dynamic message: %s", msg)

}

func TestCreateLogstashMessageWithJsonDataAndUnknownLogtype(t *testing.T) {

	assert := assert.New(t)

	m := router.Message{
		Container: &docker.Container{
			ID:   "6feffd9428dc",
			Name: "/my_app",
			Config: &docker.Config{
				Hostname: "container_hostname",
				Image:    "my.registry.host:443/path/to/image:1234",
			},
		},
		Source: "stdout",
		Data:   `{ "logtype": "nolog", "message":"here i am!", "level": "DEBUG", "file": "debug.go", "line": 42}`,
		Time:   time.Unix(int64(1453818496), 595000000),
	}

	msg, _ := createLogstashMessage(&m, "tst-mesos-slave-001", false, "my-type")
	jq := makeQuery(msg)

	assert.Equal("", getString(jq, "logtype"))

	log.Printf("Dynamic message: %s", msg)

}

func TestValidJsonMessageNoJson(t *testing.T) {
	assert := assert.New(t)

	js := `whateverthefuckever`
	assert.False(validJsonMessage(js))

}

func getString(j *jsonq.JsonQuery, s ...string) string {
	v, _ := j.String(s...)
	return v
}

func makeQuery(msg []byte) *jsonq.JsonQuery {
	data := map[string]interface{}{}
	dec := json.NewDecoder(bytes.NewReader(msg))
	dec.Decode(&data)
	jq := jsonq.NewQuery(data)
	return jq
}
