package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/ribice/golang-swaggerui-example"

	"github.com/gorilla/mux"
)

func getUserRoutes() *mux.Router {
	r := mux.NewRouter().StrictSlash(true)
	v1 := r.PathPrefix("/v1").Subrouter()
	RegisterUserRoutes(v1, "/user")
	return r
}

func TestGetUser(t *testing.T) {

	ts := httptest.NewServer(getUserRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/user"
	res, err := http.Get(path)
	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()

	var response userResponse
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", res.StatusCode)
	}
	if response.Code != http.StatusOK {
		t.Errorf("expected code OK; got %v", response.Code)

	}

	if !reflect.DeepEqual(response.Data, model.NewUser(1, "Johnny")) {
		t.Error("Expected and received data is not equal")
	}

}

func TestGetUsers(t *testing.T) {
	tt := []struct {
		name   string
		term   string
		status int
		data   []model.User
		err    string
	}{
		{name: "First letter a", term: "arn", status: http.StatusBadRequest, err: "Name starts with 'a', please change it"},
		{name: "First letter e", term: "edd", status: http.StatusForbidden, err: "Name starts with 'e', please change it"},
		{name: "Success", term: "d", status: http.StatusOK, data: model.NewUsers("danny", "don", "donald")},
	}
	ts := httptest.NewServer(getUserRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/user/search?name="

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			URL := path + tc.term
			res, err := http.Get(URL)
			if err != nil {
				t.Fatal(err)
			}

			defer res.Body.Close()

			var response usersResponse
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

func TestIsStarred(t *testing.T) {
	tt := []struct {
		name   string
		author string
		repo   string
		status int
		data   bool
		err    string
	}{
		{name: "Status forbidden", author: "ribice", repo: "go-swagger-example", status: http.StatusForbidden, err: "Author has disabled starring for this repository"},
		{name: "Status not found", author: "ribice", repo: "glice", status: http.StatusNotFound, err: "Repository does not exist"},
		{name: "Success", author: "ribice", repo: "kit", status: http.StatusOK, data: true},
	}
	ts := httptest.NewServer(getUserRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/user/starred/"

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			URL := path + tc.author + "/" + tc.repo
			res, err := http.Get(URL)
			if err != nil {
				t.Fatal(err)
			}

			defer res.Body.Close()

			var response model.BooleanResponse
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

func TestStar(t *testing.T) {
	tt := []struct {
		name   string
		author string
		repo   string
		status int
		err    string
	}{
		{name: "Status not found", author: "ribice", repo: "alpha-go", status: http.StatusNotFound, err: "Repository does not exist"},
		{name: "Success", author: "ribice", repo: "glice", status: http.StatusOK},
	}
	ts := httptest.NewServer(getUserRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/user/starred/"

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			URL := path + tc.author + "/" + tc.repo
			req, err := http.NewRequest("PUT", URL, nil)
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
				t.Errorf("expected status OK; got %v", res.StatusCode)
			}
			if response.Code != tc.status {
				t.Errorf("expected code OK; got %v", response.Code)

			}
		})
	}
}

func TestUnstar(t *testing.T) {
	tt := []struct {
		name   string
		author string
		repo   string
		status int
		err    string
	}{
		{name: "Status not found author suffix", author: "hara", repo: "obelix", status: http.StatusNotFound, err: "Repository does not exist"},
		{name: "Status not found repo prefix", author: "ribice", repo: "rorsk", status: http.StatusNotFound, err: "Repository does not exist"},
		{name: "Success", author: "ribice", repo: "glice", status: http.StatusOK},
	}
	ts := httptest.NewServer(getUserRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/user/starred/"

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			URL := path + tc.author + "/" + tc.repo
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
				t.Errorf("expected status OK; got %v", res.StatusCode)
			}
			if response.Code != tc.status {
				t.Errorf("expected code OK; got %v", response.Code)

			}
		})
	}
}

func TestIsFollowed(t *testing.T) {
	tt := []struct {
		name   string
		author string
		status int
		data   bool
		err    string
	}{
		{name: "Not following", author: "tonto", status: http.StatusOK, data: false},
		{name: "Following", author: "hara", status: http.StatusOK, data: true},
	}
	ts := httptest.NewServer(getUserRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/user/following/"

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			URL := path + tc.author
			res, err := http.Get(URL)
			if err != nil {
				t.Fatal(err)
			}

			defer res.Body.Close()

			var response model.BooleanResponse
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

			if response.Data != tc.data {
				t.Errorf("Expected %v, got %v", tc.data, response.Data)
			}
		})
	}
}

func TestFollow(t *testing.T) {
	tt := []struct {
		name   string
		author string
		status int
		err    string
	}{
		{name: "Status not found", author: "tonto", status: http.StatusNotFound},
		{name: "Success", author: "hara", status: http.StatusOK},
	}
	ts := httptest.NewServer(getUserRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/user/following/"

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			URL := path + tc.author
			req, err := http.NewRequest("PUT", URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			res, err := ts.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}

			defer res.Body.Close()

			var response model.BooleanResponse
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

		})
	}
}

func TestUnfollow(t *testing.T) {
	tt := []struct {
		name   string
		author string
		status int
		err    string
	}{
		{name: "Status not found", author: "tonto", status: http.StatusNotFound},
		{name: "Success", author: "hara", status: http.StatusOK},
	}
	ts := httptest.NewServer(getUserRoutes())
	defer ts.Close()
	path := ts.URL + "/v1/user/following/"

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			URL := path + tc.author
			req, err := http.NewRequest("DELETE", URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			res, err := ts.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}

			defer res.Body.Close()

			var response model.BooleanResponse
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

		})
	}
}
