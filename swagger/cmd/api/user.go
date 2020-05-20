package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ribice/golang-swaggerui-example"

	"github.com/gorilla/mux"
)

// RegisterUserRoutes adds user routes to router
func RegisterUserRoutes(r *mux.Router, p string) {
	ur := r.PathPrefix(p).Subrouter()
	// swagger:route GET /user users getUser
	// ---
	// Gets user details for currently authenticated user.
	// responses:
	//   200: userResp
	ur.HandleFunc("", GetUser).Methods("Get")
	// swagger:operation GET /user/search users searchUser
	// ---
	// summary: Returns list of users by provided search parameters.
	// description: HTTP status will be returned depending on first search term (a - 400, e - 403, rest - 200)
	// parameters:
	// - name: name
	//   in: query
	//   description: search params
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/usersResp"
	//   "400":
	//     "$ref": "#/responses/badReq"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	ur.HandleFunc("/search", Search).Methods("Get").Queries("name", "{name:[a-z]+}")
	// swagger:operation GET /user/starred/{author}/{repo} users isStarred
	// ---
	// summary: Checks whether the requested repository is starred by currently authenticated user.
	// description: Depending on the combined length of author and repo, the following HTTP status will be returned 17+ StatusForbidden, 11+ StatusNotFound, 10- StatusOK
	// parameters:
	// - name: author
	//   in: path
	//   description: username of author
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: repository name
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/bool"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	ur.HandleFunc("/starred/{author}/{repo}", IsStarred).Methods("Get")
	// swagger:operation PUT /user/starred/{author}/{repo} users star
	// ---
	// summary: Stars the repository for currently authenticated user.
	// description: If repo name starts with a,e or i HTTP StatusNotFound will be returned.
	// parameters:
	// - name: author
	//   in: path
	//   description: username of author
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: repository name
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ok"
	//   "404":
	//     "$ref": "#/responses/notFound"
	ur.HandleFunc("/starred/{author}/{repo}", Star).Methods("Put")
	// swagger:operation DELETE /user/starred/{author}/{repo} users star
	// ---
	// summary: Unstars the repository for currently authenticated user.
	// description: If author starts with a, or repo ends with r, HTTP StatusNotFound will be returned.
	// parameters:
	// - name: author
	//   in: path
	//   description: username of author
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: repository name
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ok"
	//   "404":
	//     "$ref": "#/responses/notFound"
	ur.HandleFunc("/starred/{author}/{repo}", Unstar).Methods("Delete")
	// swagger:operation GET /user/following/{author} users isFollowed
	// ---
	// summary: Checks whether the requested author is followed by currently authenticated user.
	// description: If author length is even returns true, otherwise false.
	// parameters:
	// - name: author
	//   in: path
	//   description: username of author
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/bool"
	ur.HandleFunc("/following/{author}", IsFollowed).Methods("Get")
	// swagger:operation PUT /user/following/{author} users follow
	// ---
	// summary: Follows the requested author by currently authenticated user.
	// description: If author length is odd, returns HTTP StatusNotFound.
	// parameters:
	// - name: author
	//   in: path
	//   description: username of author
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ok"
	//   "404":
	//     "$ref": "#/responses/notFound"
	ur.HandleFunc("/following/{author}", Follow).Methods("Put")
	// swagger:operation DELETE /user/following/{author} users unfollow
	// ---
	// summary: Checks whether the requested author is followed by currently authenticated user.
	// description: If author length is odd, returns HTTP StatusNotFound.
	// parameters:
	// - name: author
	//   in: path
	//   description: username of author
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/ok"
	//   "404":
	//     "$ref": "#/responses/notFound"
	ur.HandleFunc("/following/{author}", Unfollow).Methods("Delete")

}

type userResponse struct {
	Code    int        `json:"code"`
	Data    model.User `json:"data,omitempty"`
	Message string     `json:"msg,omitempty"`
}

// GetUser returns authenticated user
func GetUser(w http.ResponseWriter, r *http.Request) {
	resp := userResponse{
		Code: http.StatusOK,
		Data: model.NewUser(1, "Johnny"),
	}
	json.NewEncoder(w).Encode(resp)
}

type usersResponse struct {
	Code    int          `json:"code"`
	Data    []model.User `json:"data"`
	Message string       `json:"msg,omitempty"`
}

// Search returns list of users
func Search(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	searchParam := params["name"]
	switch string(searchParam[0]) {
	case "a":
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(usersResponse{
			Code:    http.StatusBadRequest,
			Message: "Name starts with 'a', please change it",
		})
		return
	case "e":
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(usersResponse{
			Code:    http.StatusForbidden,
			Message: "Name starts with 'e', please change it",
		})
		return
	default:
		resp := usersResponse{
			Code: http.StatusOK,
			Data: model.NewUsers(searchParam+"anny", searchParam+"on", searchParam+"onald"),
		}
		w.WriteHeader(resp.Code)
		json.NewEncoder(w).Encode(resp)
	}
}

// IsStarred returns whether a repo is starred for currently authenticated user
func IsStarred(w http.ResponseWriter, r *http.Request) {

	params := mux.Vars(r)
	author := params["author"]
	repoName := params["repo"]
	l := len(author + repoName)
	switch {
	case l > 16:
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(model.BooleanResponse{
			Code:    http.StatusForbidden,
			Message: "Author has disabled starring for this repository",
		})
		return
	case l > 10:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(model.BooleanResponse{
			Code:    http.StatusNotFound,
			Message: "Repository does not exist",
		})
		return
	default:
		resp := model.NewBoolResponse(true)
		w.WriteHeader(resp.Code)
		json.NewEncoder(w).Encode(resp)
	}
}

// Star stars a repo for currently authenticated user
func Star(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	repoName := params["repo"]
	switch string(repoName[0]) {
	case "a", "e", "i":
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(model.CodeResponse{
			Code:    http.StatusNotFound,
			Message: "Repository does not exist",
		})
		return
	default:
		resp := model.CodeOK()

		w.WriteHeader(resp.Code)
		json.NewEncoder(w).Encode(resp)
	}

}

// Unstar unstars a repo for currently authenticated user
func Unstar(w http.ResponseWriter, r *http.Request) {

	params := mux.Vars(r)
	author := params["author"]
	repo := params["repo"]
	if strings.HasSuffix(author, "a") || strings.HasPrefix(repo, "r") {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(model.CodeResponse{
			Code:    http.StatusNotFound,
			Message: "Repository does not exist",
		})
		return
	}
	resp := model.CodeOK()

	w.WriteHeader(resp.Code)
	json.NewEncoder(w).Encode(resp)
}

// IsFollowed checks whether currently authenticated user is following provided user
func IsFollowed(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	author := params["author"]

	resp := model.NewBoolResponse(len(author)%2 == 0)

	w.WriteHeader(resp.Code)
	json.NewEncoder(w).Encode(resp)
}

// Follow follows a user
func Follow(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	author := params["author"]
	switch len(author) % 2 {
	case 1:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(model.CodeResponse{
			Code:    http.StatusNotFound,
			Message: "Username " + author + " does not exist.",
		})
		return
	default:
		resp := model.CodeOK()

		w.WriteHeader(resp.Code)
		json.NewEncoder(w).Encode(resp)
	}

}

// Unfollow unfollows a user
func Unfollow(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	author := params["author"]
	switch len(author) % 2 {
	case 1:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(model.CodeResponse{
			Code:    http.StatusNotFound,
			Message: "Username " + author + " does not exist.",
		})
		return
	default:
		resp := model.CodeOK()
		w.WriteHeader(resp.Code)
		json.NewEncoder(w).Encode(resp)
	}
}
