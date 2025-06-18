/*
Copyright © 2025 Alexey Zapparov <alexey@zapparov.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFirstNonEmpty(t *testing.T) {
	t.Run("returns first non-empty string", func(t *testing.T) {
		assert.Equal(t, "foo", firstNonEmpty("foo", "", "bar"))
		assert.Equal(t, "foo", firstNonEmpty("", "foo", "bar"))
		assert.Equal(t, "foo", firstNonEmpty("", "", "foo"))
		assert.Equal(t, "foo", firstNonEmpty("", "foo", ""))
		assert.Equal(t, "foo", firstNonEmpty("foo", "bar", "baz"))
		assert.Equal(t, "foo", firstNonEmpty(" ", " foo ", "bar"))
		assert.Equal(t, "", firstNonEmpty("", "", ""))
		assert.Equal(t, "", firstNonEmpty())
	})
}

func TestStringStartsWith(t *testing.T) {
	t.Run("returns whenever string starts with one of given prefixes", func(t *testing.T) {
		assert.True(t, stringStartsWith("hello", "hel"))
		assert.True(t, stringStartsWith("hello", "hel", "low"))
		assert.True(t, stringStartsWith("hello", "low", "hel"))

		assert.False(t, stringStartsWith("hello", "low"))
		assert.False(t, stringStartsWith("hello"))
	})
}

func TestGetEcsTaskMetadata(t *testing.T) {
	fakeEcsTaskMetadataServer := func(t *testing.T, statusCode int, body string) *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method, "HTTP verb should be GET")

			switch path := r.URL.Path; path {
			case "/task":
				w.WriteHeader(statusCode)
				w.Write([]byte(body))

			default:
				t.Errorf("unexpected URL: %s", path)
			}
		}))

		t.Cleanup(server.Close)

		return server
	}

	t.Run("when ECS_CONTAINER_METADATA_URI_V4 is not set", func(t *testing.T) {
		os.Unsetenv("ECS_CONTAINER_METADATA_URI_V4")

		t.Run("returns empty metadata", func(t *testing.T) {
			metadata, err := getEcsTaskMetadata()

			assert.Nil(t, err, "expected no error")
			assert.NotNil(t, metadata, "expected metadata not to be nil")
			assert.Equal(t, metadata, &ecsTaskMetadata{})
		})
	})

	t.Run("when ECS_CONTAINER_METADATA_URI_V4 is set", func(t *testing.T) {
		t.Run("when server returns error", func(t *testing.T) {
			server := fakeEcsTaskMetadataServer(t, http.StatusInternalServerError, "he's not a messiah")

			os.Setenv("ECS_CONTAINER_METADATA_URI_V4", server.URL)

			metadata, err := getEcsTaskMetadata()

			assert.NotNil(t, err, "expected an error")
			assert.Nil(t, metadata, "expected metadata to be nil")
		})

		t.Run("when server returns malformed payload", func(t *testing.T) {
			server := fakeEcsTaskMetadataServer(t, http.StatusOK, "he's a very very naughty boy")

			os.Setenv("ECS_CONTAINER_METADATA_URI_V4", server.URL)

			metadata, err := getEcsTaskMetadata()

			assert.NotNil(t, err, "expected an error")
			assert.Nil(t, metadata, "expected metadata to be nil")
		})

		t.Run("when server returns valid payload with cluster name", func(t *testing.T) {
			server := fakeEcsTaskMetadataServer(t, http.StatusOK, `
				{
					"Cluster":       "cluster-name",
					"TaskARN":			 "arn:aws:ecs:aws-region-1:123456789123:task/cluster-name/deadbeef",
					"Family":        "task-family",
					"Revision":      "161",
					"ServiceName":   "service-name",
					"DesiredStatus": "RUNNING"
				}
			`)

			os.Setenv("ECS_CONTAINER_METADATA_URI_V4", server.URL)

			metadata, err := getEcsTaskMetadata()

			assert.Nil(t, err, "expected no error")
			assert.Equal(t, metadata, &ecsTaskMetadata{
				AwsRegion:       "aws-region-1",
				EcsClusterName:  "cluster-name",
				EcsServiceName:  "service-name",
				EcsTaskFamily:   "task-family",
				EcsTaskRevision: "161",
				EcsTaskARN:      "arn:aws:ecs:aws-region-1:123456789123:task/cluster-name/deadbeef",
				EcsTaskID:       "deadbeef",
			})
		})

		t.Run("when server returns valid payload with cluster name", func(t *testing.T) {
			server := fakeEcsTaskMetadataServer(t, http.StatusOK, `
				{
					"Cluster":       "arn:aws:ecs:aws-region-2:123456789123:cluster/cluster-name",
					"TaskARN":			 "arn:aws:ecs:aws-region-1:123456789123:task/cluster-name/deadbeef",
					"Family":        "task-family",
					"Revision":      "161",
					"ServiceName":   "service-name",
					"DesiredStatus": "RUNNING"
				}
			`)

			os.Setenv("ECS_CONTAINER_METADATA_URI_V4", server.URL)

			metadata, err := getEcsTaskMetadata()

			assert.Nil(t, err, "expected no error")
			assert.Equal(t, metadata, &ecsTaskMetadata{
				AwsRegion:       "aws-region-1",
				EcsClusterName:  "cluster-name",
				EcsServiceName:  "service-name",
				EcsTaskFamily:   "task-family",
				EcsTaskRevision: "161",
				EcsTaskARN:      "arn:aws:ecs:aws-region-1:123456789123:task/cluster-name/deadbeef",
				EcsTaskID:       "deadbeef",
			})
		})

		t.Run("when server returns valid payload with bogus cluster ARN", func(t *testing.T) {
			server := fakeEcsTaskMetadataServer(t, http.StatusOK, `
				{
					"Cluster":       "wazzup/cluster-name",
					"TaskARN":			 "arn:aws:ecs:aws-region-1:123456789123:task/cluster-name/deadbeef",
					"Family":        "task-family",
					"Revision":      "161",
					"ServiceName":   "service-name",
					"DesiredStatus": "RUNNING"
				}
			`)

			os.Setenv("ECS_CONTAINER_METADATA_URI_V4", server.URL)

			metadata, err := getEcsTaskMetadata()

			assert.Nil(t, err, "expected no error")
			assert.Equal(t, metadata, &ecsTaskMetadata{
				AwsRegion:       "aws-region-1",
				EcsClusterName:  "wazzup/cluster-name",
				EcsServiceName:  "service-name",
				EcsTaskFamily:   "task-family",
				EcsTaskRevision: "161",
				EcsTaskARN:      "arn:aws:ecs:aws-region-1:123456789123:task/cluster-name/deadbeef",
				EcsTaskID:       "deadbeef",
			})
		})

		t.Run("when server returns valid payload with bogus task ARN", func(t *testing.T) {
			server := fakeEcsTaskMetadataServer(t, http.StatusOK, `
				{
					"Cluster":       "cluster-name",
					"TaskARN":       "wazzup/deadbeef",
					"Family":        "task-family",
					"Revision":      "161",
					"ServiceName":   "service-name",
					"DesiredStatus": "RUNNING"
				}
			`)

			os.Setenv("ECS_CONTAINER_METADATA_URI_V4", server.URL)

			metadata, err := getEcsTaskMetadata()

			assert.Nil(t, err, "expected no error")
			assert.Equal(t, metadata, &ecsTaskMetadata{
				EcsClusterName:  "cluster-name",
				EcsServiceName:  "service-name",
				EcsTaskFamily:   "task-family",
				EcsTaskRevision: "161",
				EcsTaskARN:      "wazzup/deadbeef",
			})
		})
	})
}

func TestEcsTaskMetadata_Environ(t *testing.T) {
	resetEnviron := func(t *testing.T) {
		t.Helper()

		os.Unsetenv("AWS_REGION")
		os.Unsetenv("ECS_CLUSTER_NAME")
		os.Unsetenv("ECS_SERVICE_NAME")
		os.Unsetenv("ECS_TASK_FAMILY")
		os.Unsetenv("ECS_TASK_REVISION")
		os.Unsetenv("ECS_TASK_ARN")
		os.Unsetenv("ECS_TASK_ID")
	}

	expectedEnviron := func(env ...string) []string {
		valueFor := func(key string) string {
			for _, v := range env {
				if stringStartsWith(v, key+"=") {
					return v
				}
			}
			return key + "="
		}

		return append(
			cleanEnviron(),
			valueFor("AWS_REGION"),
			valueFor("ECS_CLUSTER_NAME"),
			valueFor("ECS_SERVICE_NAME"),
			valueFor("ECS_TASK_FAMILY"),
			valueFor("ECS_TASK_REVISION"),
			valueFor("ECS_TASK_ARN"),
			valueFor("ECS_TASK_ID"),
		)
	}

	emptyMetadata := ecsTaskMetadata{}

	t.Run("AWS_REGION", func(t *testing.T) {
		loadedMetadata := ecsTaskMetadata{AwsRegion: "deadbeef"}

		t.Run("when AWS_REGION is not set", func(t *testing.T) {
			resetEnviron(t)

			assert.Equal(t, expectedEnviron(), emptyMetadata.Environ())
			assert.Equal(t, expectedEnviron("AWS_REGION=deadbeef"), loadedMetadata.Environ())
		})

		t.Run("when AWS_REGION is set", func(t *testing.T) {
			resetEnviron(t)

			t.Setenv("AWS_REGION", "existing-value")

			assert.Equal(t, expectedEnviron("AWS_REGION=existing-value"), emptyMetadata.Environ())
			assert.Equal(t, expectedEnviron("AWS_REGION=existing-value"), loadedMetadata.Environ(),
				"does not overwrite existing AWS_REGION environment variable")
		})
	})

	t.Run("ECS_CLUSTER_NAME", func(t *testing.T) {
		loadedMetadata := ecsTaskMetadata{EcsClusterName: "deadbeef"}

		t.Run("when ECS_CLUSTER_NAME is not set", func(t *testing.T) {
			resetEnviron(t)

			assert.Equal(t, expectedEnviron(), emptyMetadata.Environ())
			assert.Equal(t, expectedEnviron("ECS_CLUSTER_NAME=deadbeef"), loadedMetadata.Environ())
		})

		t.Run("when ECS_CLUSTER_NAME is set", func(t *testing.T) {
			resetEnviron(t)

			t.Setenv("ECS_CLUSTER_NAME", "existing-value")

			assert.Equal(t, expectedEnviron("ECS_CLUSTER_NAME=existing-value"), emptyMetadata.Environ())
			assert.Equal(t, expectedEnviron("ECS_CLUSTER_NAME=existing-value"), loadedMetadata.Environ(),
				"does not overwrite existing ECS_CLUSTER_NAME environment variable")
		})
	})

	t.Run("ECS_SERVICE_NAME", func(t *testing.T) {
		loadedMetadata := ecsTaskMetadata{EcsServiceName: "deadbeef"}

		t.Run("when ECS_SERVICE_NAME is not set", func(t *testing.T) {
			resetEnviron(t)

			assert.Equal(t, expectedEnviron(), emptyMetadata.Environ())
			assert.Equal(t, expectedEnviron("ECS_SERVICE_NAME=deadbeef"), loadedMetadata.Environ())
		})

		t.Run("when ECS_SERVICE_NAME is set", func(t *testing.T) {
			resetEnviron(t)

			t.Setenv("ECS_SERVICE_NAME", "existing-value")

			assert.Equal(t, expectedEnviron("ECS_SERVICE_NAME=existing-value"), emptyMetadata.Environ())
			assert.Equal(t, expectedEnviron("ECS_SERVICE_NAME=existing-value"), loadedMetadata.Environ(),
				"does not overwrite existing ECS_SERVICE_NAME environment variable")
		})
	})

	t.Run("ECS_TASK_FAMILY", func(t *testing.T) {
		loadedMetadata := ecsTaskMetadata{EcsTaskFamily: "deadbeef"}

		t.Run("when ECS_TASK_FAMILY is not set", func(t *testing.T) {
			resetEnviron(t)

			assert.Equal(t, expectedEnviron(), emptyMetadata.Environ())
			assert.Equal(t, expectedEnviron("ECS_TASK_FAMILY=deadbeef"), loadedMetadata.Environ())
		})

		t.Run("when ECS_TASK_FAMILY is set", func(t *testing.T) {
			resetEnviron(t)

			t.Setenv("ECS_TASK_FAMILY", "existing-value")

			assert.Equal(t, expectedEnviron("ECS_TASK_FAMILY=existing-value"), emptyMetadata.Environ())
			assert.Equal(t, expectedEnviron("ECS_TASK_FAMILY=deadbeef"), loadedMetadata.Environ(),
				"overwrites existing ECS_TASK_FAMILY environment variable")
		})
	})

	t.Run("ECS_TASK_REVISION", func(t *testing.T) {
		loadedMetadata := ecsTaskMetadata{EcsTaskRevision: "161"}

		t.Run("when ECS_TASK_REVISION is not set", func(t *testing.T) {
			resetEnviron(t)

			assert.Equal(t, expectedEnviron(), emptyMetadata.Environ())
			assert.Equal(t, expectedEnviron("ECS_TASK_REVISION=161"), loadedMetadata.Environ())
		})

		t.Run("when ECS_TASK_REVISION is set", func(t *testing.T) {
			resetEnviron(t)

			t.Setenv("ECS_TASK_REVISION", "existing-value")

			assert.Equal(t, expectedEnviron("ECS_TASK_REVISION=existing-value"), emptyMetadata.Environ())
			assert.Equal(t, expectedEnviron("ECS_TASK_REVISION=161"), loadedMetadata.Environ(),
				"overwrites existing ECS_TASK_REVISION environment variable")
		})
	})

	t.Run("ECS_TASK_ARN", func(t *testing.T) {
		loadedMetadata := ecsTaskMetadata{EcsTaskARN: "arn:aws:example"}

		t.Run("when ECS_TASK_ARN is not set", func(t *testing.T) {
			resetEnviron(t)

			assert.Equal(t, expectedEnviron(), emptyMetadata.Environ())

			assert.Equal(t,
				expectedEnviron("ECS_TASK_ARN=arn:aws:example"),
				loadedMetadata.Environ(),
			)
		})

		t.Run("when ECS_TASK_ARN is set", func(t *testing.T) {
			resetEnviron(t)

			t.Setenv("ECS_TASK_ARN", "existing-value")

			assert.Equal(t, expectedEnviron("ECS_TASK_ARN=existing-value"), emptyMetadata.Environ())

			assert.Equal(t,
				expectedEnviron("ECS_TASK_ARN=arn:aws:example"),
				loadedMetadata.Environ(),
				"overwrites existing ECS_TASK_ARN environment variable",
			)
		})
	})

	t.Run("ECS_TASK_ID", func(t *testing.T) {
		loadedMetadata := ecsTaskMetadata{EcsTaskID: "deadbeef"}

		t.Run("when ECS_TASK_ID is not set", func(t *testing.T) {
			resetEnviron(t)

			assert.Equal(t, expectedEnviron(), emptyMetadata.Environ())
			assert.Equal(t, expectedEnviron("ECS_TASK_ID=deadbeef"), loadedMetadata.Environ())
		})

		t.Run("when ECS_TASK_ID is set", func(t *testing.T) {
			resetEnviron(t)

			t.Setenv("ECS_TASK_ID", "existing-value")

			assert.Equal(t, expectedEnviron("ECS_TASK_ID=existing-value"), emptyMetadata.Environ())

			assert.Equal(t,
				expectedEnviron("ECS_TASK_ID=deadbeef"),
				loadedMetadata.Environ(),
				"overwrites existing ECS_TASK_ID environment variable",
			)
		})
	})
}
