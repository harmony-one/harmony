package utils

import (
	"bufio"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

func DownloadFile(filepath string, url string) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func ReadDistributionConfig(filename string) ([][]string, error) {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		log.Fatal("Failed to read config file ", filename)
		return nil, err
	}
	fscanner := bufio.NewScanner(file)

	result := [][]string{}
	for fscanner.Scan() {
		p := strings.Split(fscanner.Text(), " ")
		result = append(result, p)
	}
	log.Println(result)
	return result, nil
}
