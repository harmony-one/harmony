package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bouk/monkey"
	"github.com/gorilla/mux"

	"github.com/ribice/golang-swaggerui-example"
)

func getRepoRoutes() *mux.Router {
	r := mux.NewRouter().StrictSlash(true)
	v1 := r.PathPrefix("/v1").Subrouter()
	RegisterRepoRoutes(v1, "/repo")
	return r
}

func TestGetRepos(t *testing.T) {
	tt := []struct {
		name   string
		author string
		status int
		data   []model.Repository
		err    string
	}{
		{name: "author length 5", author: "tonto", status: http.StatusOK, data: model.NewRepositories("tonto")},
		{name: "author length 6", author: "ribice", status: http.StatusNotFound, err: "Author does not exist"},
	}
	ts := httptest.NewServer(getRepoRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/repo/"

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			URL := path + tc.author
			res, err := http.Get(URL)
			if err != nil {
				t.Fatal(err)
			}

			defer res.Body.Close()

			var response reposResponse
			err = json.NewDecoder(res.Body).Decode(&response)
			if err != nil {
				t.Fatal(err)
			}

			if tc.err != "" {
				if res.StatusCode != tc.status {
					t.Errorf("expected status %v; got %v", tc.status, res.StatusCode)
				}
				if response.Message != tc.err {
					t.Errorf("expected message %q; got %q", tc.err, response.Message)
				}
				if response.Code != tc.status {
					t.Errorf("expected code %v; got %v", tc.status, response.Code)
				}
				return
			}

			if res.StatusCode != tc.status {
				t.Errorf("expected status OK; got %v", res.StatusCode)
			}
			if response.Code != tc.status {
				t.Errorf("expected code OK; got %v", response.Code)

			}

			if !reflect.DeepEqual(response.Data, tc.data) {
				t.Error("Expected and received data is not equal", response.Data, tc.data)
			}
		})
	}

}

func TestGetRepo(t *testing.T) {
	tt := []struct {
		name   string
		author string
		repo   string
		status int
		data   model.Repository
		err    string
	}{
		{name: "even author and repo length", author: "tonto", repo: "kit", status: http.StatusOK, data: model.NewRepository(1, "kit", "tonto")},
		{name: "odd author and repo length", author: "ribice", repo: "glice", status: http.StatusNotFound, err: "Author or repository does not exist"},
	}
	ts := httptest.NewServer(getRepoRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/repo/"

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			URL := path + tc.author + "/" + tc.repo
			res, err := http.Get(URL)
			if err != nil {
				t.Fatal(err)
			}

			defer res.Body.Close()

			var response repoResponse
			err = json.NewDecoder(res.Body).Decode(&response)
			if err != nil {
				t.Fatal(err)
			}

			if tc.err != "" {
				if res.StatusCode != tc.status {
					t.Errorf("expected status %v; got %v", tc.status, res.StatusCode)
				}
				if response.Message != tc.err {
					t.Errorf("expected message %q; got %q", tc.err, response.Message)
				}
				if response.Code != tc.status {
					t.Errorf("expected code %v; got %v", tc.status, response.Code)
				}
				return
			}

			if res.StatusCode != tc.status {
				t.Errorf("expected status %v; got %v", tc.status, res.StatusCode)
			}
			if response.Code != tc.status {
				t.Errorf("expected code %v; got %v", tc.status, response.Code)
			}
			if !reflect.DeepEqual(response.Data, tc.data) {
				t.Error("Expected and received data is not equal", response.Data, tc.data)
			}
		})
	}

}

func TestCreateRepo(t *testing.T) {
	tt := []struct {
		name   string
		req    string
		status int
		data   model.Repository
		err    string
	}{
		{name: "Empty request", status: http.StatusBadRequest, err: "unexpected end of JSON input"},
		{name: "Invalid JSON", req: `{"testing":"true"}`, status: http.StatusBadRequest, err: "Name cannot be empty"},
		{name: "Repository exists", req: `{"name":"exists", "public":true}`, status: http.StatusConflict, err: "Repository with requested name already exists"},
		{name: "Success", req: `{"name":"glice","public":false}`, status: http.StatusOK, data: model.NewRepository(1, "glice", "John")},
	}
	ts := httptest.NewServer(getRepoRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/repo"

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res, err := http.Post(path, "application/json", strings.NewReader(tc.req))
			if err != nil {
				t.Fatal(err)
			}

			defer res.Body.Close()

			var response repoResponse
			err = json.NewDecoder(res.Body).Decode(&response)
			if err != nil {
				t.Fatal(err)
			}

			if tc.err != "" {
				if res.StatusCode != tc.status {
					t.Errorf("expected status %v; got %v", tc.status, res.StatusCode)
				}
				if response.Message != tc.err {
					t.Errorf("expected message %q; got %q", tc.err, response.Message)
				}
				if response.Code != tc.status {
					t.Errorf("expected code %v; got %v", tc.status, response.Code)
				}
				return
			}

			if res.StatusCode != tc.status {
				t.Errorf("expected status %v; got %v", tc.status, res.StatusCode)
			}
			if response.Code != tc.status {
				t.Errorf("expected code %v; got %v", tc.status, response.Code)
			}
			if !reflect.DeepEqual(response.Data, tc.data) {
				t.Error("Expected and received data is not equal", response.Data, tc.data)
			}

		})
	}

}

func TestUpdateRepo(t *testing.T) {
	layout := "2006-01-02T15:04:05.000Z"
	et, _ := time.Parse(layout, "0001-01-01T00:00:00Z")

	tt := []struct {
		name   string
		req    string
		status int
		data   model.Repository
		err    string
	}{
		{name: "Invalid JSON", status: http.StatusBadRequest, err: "unexpected end of JSON input"},
		{name: "Invalid JSON content", req: `{"testing":"true"}`, status: http.StatusBadRequest, err: "Invalid request"},
		{name: "Success", req: `{"name":"glice","owner":{}}`, status: http.StatusOK, data: model.Repository{
			Name:      "glice",
			CreatedOn: et,
			UpdatedOn: time.Date(2018, time.January, 03, 1, 2, 3, 0, time.UTC),
			DeletedOn: et,
			Owner: model.User{
				LastLogin: et,
				CreatedOn: et,
				UpdatedOn: et,
				DeletedOn: et,
			},
		}},
	}
	ts := httptest.NewServer(getRepoRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/repo"
	wayback := time.Date(1974, time.May, 19, 1, 2, 3, 4, time.UTC)
	patch := monkey.Patch(time.Now, func() time.Time { return wayback })
	defer patch.Unpatch()

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest("PUT", path, strings.NewReader(tc.req))
			if err != nil {
				t.Fatal(err)
			}
			res, err := ts.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}

			defer res.Body.Close()

			var response repoResponse
			err = json.NewDecoder(res.Body).Decode(&response)
			if err != nil {
				t.Fatal(err)
			}

			if tc.err != "" {
				if res.StatusCode != tc.status {
					t.Errorf("expected status %v; got %v", tc.status, res.StatusCode)
				}
				if response.Message != tc.err {
					t.Errorf("expected message %q; got %q", tc.err, response.Message)
				}
				if response.Code != tc.status {
					t.Errorf("expected code %v; got %v", tc.status, response.Code)
				}
				return
			}

			if res.StatusCode != tc.status {
				t.Errorf("expected status %v; got %v", tc.status, res.StatusCode)
			}
			if response.Code != tc.status {
				t.Errorf("expected code %v; got %v", tc.status, response.Code)
			}
			if !reflect.DeepEqual(response.Data, tc.data) {
				t.Error("Expected and received data is not equal", response.Data, tc.data)
			}

		})
	}

}

func TestDeleteRepo(t *testing.T) {
	tt := []struct {
		name   string
		repo   string
		status int
		err    string
	}{
		{name: "Unexisting repo", repo: "kiss", status: http.StatusNotFound, err: "Repo does not exist"},
		{name: "Protected repo", repo: "glice", status: http.StatusForbidden, err: "This repository has protected status. Disable it in settings to proceed with deletion."},
		{name: "Success", repo: "kit", status: http.StatusOK},
	}
	ts := httptest.NewServer(getRepoRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/repo/"

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			URL := path + tc.repo
			req, err := http.NewRequest("DELETE", URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			res, err := ts.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}

			defer res.Body.Close()

			var response model.CodeResponse
			err = json.NewDecoder(res.Body).Decode(&response)
			if err != nil {
				t.Fatal(err)
			}

			if tc.err != "" {
				if res.StatusCode != tc.status {
					t.Errorf("expected status %v; got %v", tc.status, res.StatusCode)
				}
				if response.Message != tc.err {
					t.Errorf("expected message %q; got %q", tc.err, response.Message)
				}
				if response.Code != tc.status {
					t.Errorf("expected code %v; got %v", tc.status, response.Code)
				}
				return
			}

			if res.StatusCode != tc.status {
				t.Errorf("expected status %v; got %v", tc.status, res.StatusCode)
			}
			if response.Code != tc.status {
				t.Errorf("expected code %v; got %v", tc.status, response.Code)
			}

		})
	}

}
