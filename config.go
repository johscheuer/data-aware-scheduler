package main

import (
	"io/ioutil"
	"log"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"
)

type schedulerConfig struct {
	Kubeconfig string
	Backend    string
	Opts       map[string]string
}

func readConfig(configPath string) schedulerConfig {
	filename, _ := filepath.Abs(configPath)
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalln(err)
	}

	var config schedulerConfig
	if err = yaml.Unmarshal(yamlFile, &config); err != nil {
		log.Fatalln(err)
	}

	return config
}
