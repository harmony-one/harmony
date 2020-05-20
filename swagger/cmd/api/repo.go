package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/ribice/golang-swaggerui-example"

	"github.com/gorilla/mux"
)

// RegisterRepoRoutes adds repo routes to router
func RegisterRepoRoutes(r *mux.Router, p string) {
	rr := r.PathPrefix(p).Subrouter()
	// swagger:operation GET /repo/{author} repos repoList
	// ---
	// summary: List the repositories owned by the given author.
	// description: If author length is between 6 and 8, Error Not Found (404) will be returned.
	// parameters:
	// - name: author
	//   in: path
	//   description: username of author
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/reposResp"
	//   "400":
	//     "$ref": "#/responses/badReq"
	//   "404":
	//     "$ref": "#/responses/notFound"
	rr.HandleFunc("/{author}", GetRepos).Methods("GET")
	// swagger:operation GET /repo/{author}/{repo} repos getRepo
	// ---
	// summary: Returns requested repository.
	// description: If length of author and repo combined is an odd number, Error Not Found (404) will be returned.
	// parameters:
	// - name: author
	//   in: path
	//   description: username of author
	//   type: string
	//   required:
	// responses:
	//   "200":
	//     "$ref": "#/responses/repoResp"
	//   "404":
	//     "$ref": "#/responses/notFound"
	rr.HandleFunc("/{author}/{repo}", GetRepo).Methods("GET")
	// swagger:route POST /repo repos createRepoReq
	// Creates a new repository for the currently authenticated user.
	// If repository name is "exists", error conflict (409) will be returned.
	// responses:
	//  200: repoResp
	//  400: badReq
	//  409: conflict
	rr.HandleFunc("", CreateRepo).Methods("POST")
	// swagger:route PUT /repo repos repoReq
	// Updates an existing repository for currently authenticated user.
	// responses:
	//   200: repoResp
	//   400: badReq
	rr.HandleFunc("", UpdateRepo).Methods("PUT")
	// swagger:operation DELETE /repo/{repo} repos deleteRepo
	// ---
	// summary: Deletes requested repo if the owner is currently authenticated.
	// description: Depending on the repository name modulo three, HTTP Status Forbidden (403), HTTP Status Not Found (404) or HTTP Status OK (200) may be returned.
	// parameters:
	// - name: repo
	//   in: path
	//   description: repository name
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ok"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	rr.HandleFunc("/{repo}", DeleteRepo).Methods("DELETE")

}

type reposResponse struct {
	Code    int                `json:"code"`
	Data    []model.Repository `json:"data,omitempty"`
	Message string             `json:"msg,omitempty"`
}

// GetRepos returns list of repositories from user
func GetRepos(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	author := params["author"]
	switch len(author) {
	case 6, 7, 8:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(reposResponse{
			Code:    http.StatusNotFound,
			Message: "Author does not exist",
		})
		return
	default:
		resp := reposResponse{
			Code: http.StatusOK,
			Data: model.NewRepositories(author),
		}
		w.WriteHeader(resp.Code)
		json.NewEncoder(w).Encode(resp)
	}

}

type repoResponse struct {
	Code    int              `json:"code"`
	Data    model.Repository `json:"data"`
	Message string           `json:"msg,omitempty"`
}

// GetRepo returns repository information by provided owner and repo name
func GetRepo(w http.ResponseWriter, r *http.Request) {

	params := mux.Vars(r)
	author := params["author"]
	repo := params["repo"]
	switch len(author+repo) % 2 {
	case 1:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(repoResponse{
			Code:    http.StatusNotFound,
			Message: "Author or repository does not exist",
		})
		return
	default:
		resp := repoResponse{
			Code: http.StatusOK,
			Data: model.NewRepository(1, repo, author),
		}
		w.WriteHeader(resp.Code)
		json.NewEncoder(w).Encode(resp)
	}
}

// CreateRepoReq contains request data for create repo API
type CreateRepoReq struct {
	// Name of the repository
	Name string `json:"name"`
	// Public defines whether created repository should be public or not
	Public bool `json:"public"`
}

// CreateRepo creates a new repo
func CreateRepo(w http.ResponseWriter, r *http.Request) {
	var req CreateRepoReq

	body, _ := ioutil.ReadAll(r.Body)

	err := json.Unmarshal(body, &req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(repoResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	if req.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(repoResponse{
			Code:    http.StatusBadRequest,
			Message: "Name cannot be empty",
		})
		return
	}
	if req.Name == "exists" {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(repoResponse{
			Code:    http.StatusConflict,
			Message: "Repository with requested name already exists",
		})
		return
	}
	resp := repoResponse{
		Code: http.StatusOK,
		Data: model.NewRepository(1, req.Name, "John"),
	}

	w.WriteHeader(resp.Code)
	json.NewEncoder(w).Encode(resp)
}

// UpdateRepo updates an existing repo
func UpdateRepo(w http.ResponseWriter, r *http.Request) {

	body, _ := ioutil.ReadAll(r.Body)

	var repo model.Repository
	err := json.Unmarshal(body, &repo)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(repoResponse{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}
	if repo.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(repoResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid request",
		})
		return
	}

	repo.UpdatedOn = time.Date(2018, time.January, 03, 1, 2, 3, 0, time.UTC)

	resp := repoResponse{
		Code: http.StatusOK,
		Data: repo,
	}
	w.WriteHeader(resp.Code)
	json.NewEncoder(w).Encode(resp)
}

// DeleteRepo deletes a repo
func DeleteRepo(w http.ResponseWriter, r *http.Request) {

	params := mux.Vars(r)
	repo := params["repo"]
	switch len(repo) % 3 {
	case 1:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(repoResponse{
			Code:    http.StatusNotFound,
			Message: "Repo does not exist",
		})
		return
	case 2:
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(repoResponse{
			Code:    http.StatusForbidden,
			Message: "This repository has protected status. Disable it in settings to proceed with deletion.",
		})
		return
	default:
		c := model.CodeOK()
		w.WriteHeader(c.Code)
		json.NewEncoder(w).Encode(c)
	}

}
