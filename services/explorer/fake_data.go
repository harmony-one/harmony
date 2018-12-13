package explorer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

var (
	data = ReadFakeData()
)

// ReadFakeData ...
func ReadFakeData() Data {
	jsonFile, err := os.Open("./fake_data.json")
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Successfully Opened users.json")
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()

	// read our opened xmlFile as a byte array.
	byteValue, _ := ioutil.ReadAll(jsonFile)

	// we initialize our Users array
	var data Data

	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'users' which we defined above
	json.Unmarshal(byteValue, &data)
	return data
}
