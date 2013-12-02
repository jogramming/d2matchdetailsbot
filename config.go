package main

import (
	"encoding/json"
	"io/ioutil"
)

type Config struct {
	RUser string
	RPass string
	RSub  string

	D2Key string
}

func LoadConfig(path string) (*Config, error) {
	b, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(b, &config)

	return &config, err
}
