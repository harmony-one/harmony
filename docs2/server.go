package docs

import (
	"log"
	"net/http"

	"github.com/rakyll/statik/fs"
	_ "github.com/harmony-one/harmony/docs/swaggerui" // statik files

	"github.com/gorilla/mux"
)

func docServer() {
	router := mux.NewRouter().StrictSlash(true)
	statikFS, err := fs.New()
	if err != nil {
		panic(err)
	}

	staticServer := http.FileServer(statikFS)
	sh := http.StripPrefix("/swaggerui/", staticServer)
	router.PathPrefix("/swaggerui/").Handler(sh)
//	registerV1Routes(router)
	log.Fatal(http.ListenAndServe(":8180", router))
}

/*
func registerV1Routes(r *mux.Router) {
	v1 := r.PathPrefix("/v1").Subrouter()
	api.RegisterRepoRoutes(v1, "/repo")
	api.RegisterUserRoutes(v1, "/user")
}
*/
