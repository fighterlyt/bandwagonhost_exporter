package main

import "time"

type Config struct {
	Hosts          []HostConfig  `yaml:"hosts"`
	ScrapeInterval time.Duration `yaml:"scrapeInterval"`
	Port           uint          `yaml:"port"`
}

type HostConfig struct {
	Veid   string `yaml:"veid"`
	APIKey string `yaml:"apiKey"`
	Name   string `yaml:"name"`
}
