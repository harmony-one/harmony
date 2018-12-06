package explorer_test

// http://www.golangprograms.com/golang-restful-api-using-grom-and-gorilla-mux.html
// https://dev.to/codehakase/building-a-restful-api-with-go
// https://thenewstack.io/make-a-restful-json-api-go/
// https://medium.com/@kelvin_sp/building-and-testing-a-rest-api-in-golang-using-gorilla-mux-and-mysql-1f0518818ff6
// var a App

// func TestMain(m *testing.M) {
// 	a = App{}
// 	a.Initialize("root", "", "rest_api_example")

// 	ensureTableExists()

// 	code := m.Run()

// 	clearTable()

// 	os.Exit(code)
// }

// func ensureTableExists() {
// 	if _, err := a.DB.Exec(tableCreationQuery); err != nil {
// 		log.Fatal(err)
// 	}
// }

// func clearTable() {
// 	a.DB.Exec("DELETE FROM users")
// 	a.DB.Exec("ALTER TABLE users AUTO_INCREMENT = 1")
// }

// func executeRequest(req *http.Request) *httptest.ResponseRecorder {
// 	rr := httptest.NewRecorder()
// 	a.Router.ServeHTTP(rr, req)

// 	return rr
// }

// func checkResponseCode(t *testing.T, expected, actual int) {
// 	if expected != actual {
// 		t.Errorf("Expected response code %d. Got %d\n", expected, actual)
// 	}
// }

// func TestGetNonExistentUser(t *testing.T) {
// 	clearTable()

// 	req, _ := http.NewRequest("GET", "/user/45", nil)
// 	response := executeRequest(req)

// 	checkResponseCode(t, http.StatusNotFound, response.Code)

// 	var m map[string]string
// 	json.Unmarshal(response.Body.Bytes(), &m)
// 	if m["error"] != "User not found" {
// 		t.Errorf("Expected the 'error' key of the response to be set to 'User not found'. Got '%s'", m["error"])
// 	}
// }

// func TestCreateUser(t *testing.T) {
// 	clearTable()

// 	payload := []byte(`{"name":"test user","age":30}`)

// 	req, _ := http.NewRequest("POST", "/user", bytes.NewBuffer(payload))
// 	response := executeRequest(req)

// 	checkResponseCode(t, http.StatusCreated, response.Code)

// 	var m map[string]interface{}
// 	json.Unmarshal(response.Body.Bytes(), &m)

// 	if m["name"] != "test user" {
// 		t.Errorf("Expected user name to be 'test user'. Got '%v'", m["name"])
// 	}

// 	if m["age"] != 30.0 {
// 		t.Errorf("Expected user age to be '30'. Got '%v'", m["age"])
// 	}

// 	// the id is compared to 1.0 because JSON unmarshaling converts numbers to
// 	// floats, when the target is a map[string]interface{}
// 	if m["id"] != 1.0 {
// 		t.Errorf("Expected product ID to be '1'. Got '%v'", m["id"])
// 	}
// }

// func addUsers(count int) {
// 	if count < 1 {
// 		count = 1
// 	}

// 	for i := 0; i < count; i++ {
// 		statement := fmt.Sprintf("INSERT INTO users(name, age) VALUES('%s', %d)", ("User " + strconv.Itoa(i+1)), ((i + 1) * 10))
// 		a.DB.Exec(statement)
// 	}
// }

// func TestGetUser(t *testing.T) {
// 	clearTable()
// 	addUsers(1)

// 	req, _ := http.NewRequest("GET", "/user/1", nil)
// 	response := executeRequest(req)

// 	checkResponseCode(t, http.StatusOK, response.Code)
// }

// func TestUpdateUser(t *testing.T) {
// 	clearTable()
// 	addUsers(1)

// 	req, _ := http.NewRequest("GET", "/user/1", nil)
// 	response := executeRequest(req)
// 	var originalUser map[string]interface{}
// 	json.Unmarshal(response.Body.Bytes(), &originalUser)

// 	payload := []byte(`{"name":"test user - updated name","age":21}`)

// 	req, _ = http.NewRequest("PUT", "/user/1", bytes.NewBuffer(payload))
// 	response = executeRequest(req)

// 	checkResponseCode(t, http.StatusOK, response.Code)

// 	var m map[string]interface{}
// 	json.Unmarshal(response.Body.Bytes(), &m)

// 	if m["id"] != originalUser["id"] {
// 		t.Errorf("Expected the id to remain the same (%v). Got %v", originalUser["id"], m["id"])
// 	}

// 	if m["name"] == originalUser["name"] {
// 		t.Errorf("Expected the name to change from '%v' to '%v'. Got '%v'", originalUser["name"], m["name"], m["name"])
// 	}

// 	if m["age"] == originalUser["age"] {
// 		t.Errorf("Expected the age to change from '%v' to '%v'. Got '%v'", originalUser["age"], m["age"], m["age"])
// 	}
// }

// func TestDeleteUser(t *testing.T) {
// 	clearTable()
// 	addUsers(1)

// 	req, _ := http.NewRequest("GET", "/user/1", nil)
// 	response := executeRequest(req)
// 	checkResponseCode(t, http.StatusOK, response.Code)

// 	req, _ = http.NewRequest("DELETE", "/user/1", nil)
// 	response = executeRequest(req)

// 	checkResponseCode(t, http.StatusOK, response.Code)

// 	req, _ = http.NewRequest("GET", "/user/1", nil)
// 	response = executeRequest(req)
// 	checkResponseCode(t, http.StatusNotFound, response.Code)
// }
