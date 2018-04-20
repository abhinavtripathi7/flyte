/*
Copyright (C) 2018 Expedia Group.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pack

import (
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"github.com/HotelsDotCom/flyte/flytepath"
	"github.com/HotelsDotCom/flyte/httputil"
	"github.com/HotelsDotCom/go-logger/loggertest"
	"strings"
	"testing"
)

func TestPostPack_ShouldCreatePackForValidRequest(t *testing.T) {

	defer resetPackRepo()
	actualPack := Pack{}
	packRepo = mockPackRepo{
		add: func(pack Pack) error {
			actualPack = pack
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/packs", strings.NewReader(packRequest))
	httputil.SetProtocolAndHostIn(req)
	w := httptest.NewRecorder()
	PostPack(w, req)

	resp := w.Result()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	location, err := resp.Location()
	require.NoError(t, err)
	assert.Equal(t, "http://example.com/v1/packs/Slack", location.String())

	var expectedPack Pack
	err = json.Unmarshal([]byte(packRequest), &expectedPack)
	require.NoError(t, err)
	expectedPack.generateId()
	assert.Equal(t, expectedPack, actualPack)
}

func TestPostPack_ShouldReturn400ForInvalidRequest(t *testing.T) {

	defer loggertest.Reset()
	loggertest.Init(loggertest.LogLevelError)

	req := httptest.NewRequest(http.MethodPost, "/v1/packs", strings.NewReader(`--- invalid json ---`))
	w := httptest.NewRecorder()
	PostPack(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	logMessages := loggertest.GetLogMessages()
	require.Len(t, logMessages, 1)
	assert.Equal(t, "Cannot convert request to pack: invalid character '-' in numeric literal", logMessages[0].Message)
}

func TestPostPack_ShouldReturn500_WhenRepoFails(t *testing.T) {

	defer loggertest.Reset()
	loggertest.Init(loggertest.LogLevelError)

	defer resetPackRepo()
	packRepo = mockPackRepo{
		add: func(pack Pack) error {
			return errors.New("something went wrong")
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/packs", strings.NewReader(packRequest))
	w := httptest.NewRecorder()
	PostPack(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	logMessages := loggertest.GetLogMessages()
	require.Len(t, logMessages, 1)
	assert.Equal(t, "Cannot save packName=Slack, packLabels=map[]: something went wrong", logMessages[0].Message)
}

func TestGetPacks_ShouldReturnListOfPacksWithLinks_WhenPacksExist(t *testing.T) {

	defer resetPackRepo()
	packRepo = mockPackRepo{
		findAll: func() ([]Pack, error) {
			slack := Pack{Id: "Slack", Name: "Slack", Labels: map[string]string{"env": "dev"}}
			hipChat := Pack{Id: "HipChat", Name: "HipChat"}
			return []Pack{slack, hipChat}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/packs", nil)
	httputil.SetProtocolAndHostIn(req)
	flytepath.EnsureUriDocMapIsInitialised(req)
	w := httptest.NewRecorder()
	GetPacks(w, req)

	resp := w.Result()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, httputil.ContentTypeJson, resp.Header.Get(httputil.HeaderContentType))
	assert.Equal(t, slackAndHipchatPacksResponse, string(body))
}

func TestGetPacks_ShouldReturnEmptyListOfPacksWithLinks_WhenThereAreNoPacks(t *testing.T) {

	defer resetPackRepo()
	packRepo = mockPackRepo{
		findAll: func() ([]Pack, error) {
			return []Pack{}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/packs", nil)
	httputil.SetProtocolAndHostIn(req)
	flytepath.EnsureUriDocMapIsInitialised(req)
	w := httptest.NewRecorder()
	GetPacks(w, req)

	resp := w.Result()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, httputil.ContentTypeJson, resp.Header.Get(httputil.HeaderContentType))
	assert.Equal(t, emptyPacksResponse, string(body))
}

func TestGetPacks_ShouldReturn500_WhenRepoFails(t *testing.T) {

	defer loggertest.Reset()
	loggertest.Init(loggertest.LogLevelError)

	defer resetPackRepo()
	packRepo = mockPackRepo{
		findAll: func() ([]Pack, error) {
			return nil, errors.New("something went wrong")
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/packs", nil)
	w := httptest.NewRecorder()
	GetPacks(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	logMessages := loggertest.GetLogMessages()
	require.Len(t, logMessages, 1)
	assert.Equal(t, "Cannot find packs: something went wrong", logMessages[0].Message)
}

func TestGetPack_ShouldReturnPack(t *testing.T) {

	defer resetPackRepo()
	packRepo = mockPackRepo{
		get: func(id string) (*Pack, error) {
			if id == "Slack" {
				slack := &Pack{}
				json.NewDecoder(strings.NewReader(slackPackJson)).Decode(slack)
				slack.Id = "Slack"
				return slack, nil
			}
			return nil, PackNotFoundErr
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/Slack?:packId=Slack", nil)
	httputil.SetProtocolAndHostIn(req)
	flytepath.EnsureUriDocMapIsInitialised(req)
	w := httptest.NewRecorder()
	GetPack(w, req)

	resp := w.Result()
	body, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, slackPackResponse, string(body))
}

func TestGetPack_Should404ForNonExistingPack(t *testing.T) {

	defer resetPackRepo()
	packRepo = mockPackRepo{
		get: func(id string) (*Pack, error) {
			return nil, PackNotFoundErr
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/Slack", nil)
	w := httptest.NewRecorder()
	GetPack(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetPack_Should500_WhenRepoFails(t *testing.T) {

	defer loggertest.Reset()
	loggertest.Init(loggertest.LogLevelError)

	defer resetPackRepo()
	packRepo = mockPackRepo{
		get: func(id string) (*Pack, error) {
			return nil, errors.New("something went wrong")
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/packs/Slack", nil)
	w := httptest.NewRecorder()
	GetPack(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	logMessages := loggertest.GetLogMessages()
	require.Len(t, logMessages, 1)
	assert.Equal(t, "Cannot find packId=: something went wrong", logMessages[0].Message)
}

func TestDeletePack_ShouldDeleteExistingPack(t *testing.T) {

	defer resetPackRepo()
	packRepo = mockPackRepo{
		remove: func(id string) error {
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/packs/Slack", nil)
	w := httptest.NewRecorder()
	DeletePack(w, req)

	resp := w.Result()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Empty(t, string(body))
}

func TestDeletePack_Should404ForNonExistingPack(t *testing.T) {

	defer resetPackRepo()
	packRepo = mockPackRepo{
		remove: func(id string) error {
			return PackNotFoundErr
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/packs/Slack", nil)
	w := httptest.NewRecorder()
	DeletePack(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeletePack_Should500_WhenRepoFails(t *testing.T) {

	defer loggertest.Reset()
	loggertest.Init(loggertest.LogLevelError)

	defer resetPackRepo()
	packRepo = mockPackRepo{
		remove: func(id string) error {
			return errors.New("something went wrong")
		},
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/packs/Slack", nil)
	w := httptest.NewRecorder()
	DeletePack(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	logMessages := loggertest.GetLogMessages()
	require.Len(t, logMessages, 1)
	assert.Equal(t, "Cannot delete packId=: something went wrong", logMessages[0].Message)
}

// --- requests/responses ---

var packRequest = `
{
    "name": "Slack",
    "commands": [
        {
            "name": "SendMessage",
            "events": ["MessageSent", "SendMessageFailed"]
        }
    ],
    "events": [
        {
            "name": "MessageSent"
        },
        {
            "name": "SendMessageFailed"
        }
    ]
}
`

var slackPackJson = `
{
    "name": "Slack",
    "commands": [
        {
            "name": "SendMessage",
            "events": ["MessageSent", "SendMessageFailed"]
        }
    ],
    "events": [
        {
            "name": "MessageSent"
        },
        {
            "name": "SendMessageFailed"
        }
    ],
	"links": [
		{
			"href": "http://example.com/README.md",
			"rel": "help"
		}
	]
}
`

var slackPackResponse = strings.Replace(strings.Replace(`
{
    "id": "Slack",
    "name": "Slack",
    "commands": [
        {
            "name": "SendMessage",
            "events": ["MessageSent", "SendMessageFailed"],
            "links": [
                {
                    "href": "http://example.com/v1/packs/Slack/actions/take?commandName=SendMessage",
                    "rel": "http://example.com/swagger#!/action/takeAction"
                }
            ]
        }
    ],
    "events": [
        {
            "name": "MessageSent"
        },
        {
            "name": "SendMessageFailed"
        }
    ],
    "links": [
        {
            "href": "http://example.com/README.md",
            "rel": "help"
        },
        {
            "href": "http://example.com/v1/packs/Slack",
            "rel": "self"
        },
        {
            "href": "http://example.com/v1/packs",
            "rel": "up"
        },
        {
            "href": "http://example.com/v1/packs/Slack/actions/take",
            "rel": "http://example.com/swagger#!/action/takeAction"
        },
        {
            "href": "http://example.com/v1/packs/Slack/events",
            "rel": "http://example.com/swagger#/event"
        }
    ]
}
`, "\n", "", -1), " ", "", -1)

var slackAndHipchatPacksResponse = strings.Replace(strings.Replace(`
{
    "links": [
        {
            "href": "http://example.com/v1/packs",
            "rel": "self"
        },
        {
            "href": "http://example.com/v1",
            "rel": "up"
        },
        {
            "href": "http://example.com/swagger#/pack",
            "rel": "help"
        }
    ],
    "packs": [
        {
            "id": "Slack",
            "name": "Slack",
            "labels": {
                "env": "dev"
            },
            "links": [
                {
                    "href": "http://example.com/v1/packs/Slack",
                    "rel": "self"
                }
            ]
        },
        {
            "id": "HipChat",
            "name": "HipChat",
            "links": [
                {
                    "href": "http://example.com/v1/packs/HipChat",
                    "rel": "self"
                }
            ]
        }
    ]
}
`, "\n", "", -1), " ", "", -1)

var emptyPacksResponse = strings.Replace(strings.Replace(`
{
    "links": [
        {
            "href": "http://example.com/v1/packs",
            "rel": "self"
        },
        {
            "href": "http://example.com/v1",
            "rel": "up"
        },
        {
            "href": "http://example.com/swagger#/pack",
            "rel": "help"
        }
    ],
    "packs": []
}
`, "\n", "", -1), " ", "", -1)

// --- mocks & helpers ---

type mockPackRepo struct {
	add     func(pack Pack) error
	remove  func(id string) error
	get     func(id string) (*Pack, error)
	findAll func() ([]Pack, error)
}

func (r mockPackRepo) Add(pack Pack) error {
	return r.add(pack)
}

func (r mockPackRepo) Remove(id string) error {
	return r.remove(id)
}

func (r mockPackRepo) Get(id string) (*Pack, error) {
	return r.get(id)
}

func (r mockPackRepo) FindAll() ([]Pack, error) {
	return r.findAll()
}

func resetPackRepo() {
	packRepo = packMgoRepo{}
}