package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Warrior struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type Config struct {
	ListenAddr          string        `yaml:"listen_addr"`
	Nickname            string        `yaml:"nickname"`
	Warriors            []Warrior     `yaml:"warriors"`
	LeaderboardInterval time.Duration `yaml:"leaderboard_interval"`
	ReconnectMin        time.Duration `yaml:"reconnect_min"`
	ReconnectMax        time.Duration `yaml:"reconnect_max"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	c.applyDefaults()
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.ListenAddr == "" {
		c.ListenAddr = ":8080"
	}
	if c.LeaderboardInterval == 0 {
		c.LeaderboardInterval = 5 * time.Minute
	}
	if c.ReconnectMin == 0 {
		c.ReconnectMin = time.Second
	}
	if c.ReconnectMax == 0 {
		c.ReconnectMax = 60 * time.Second
	}
}

func (c *Config) validate() error {
	if c.Nickname == "" {
		return fmt.Errorf("nickname is required")
	}
	if len(c.Warriors) == 0 {
		return fmt.Errorf("at least one warrior is required")
	}
	seen := make(map[string]bool, len(c.Warriors))
	for i, w := range c.Warriors {
		if w.Name == "" {
			return fmt.Errorf("warriors[%d]: name is required", i)
		}
		if w.URL == "" {
			return fmt.Errorf("warriors[%d] (%s): url is required", i, w.Name)
		}
		if seen[w.Name] {
			return fmt.Errorf("warriors[%d]: duplicate name %q", i, w.Name)
		}
		seen[w.Name] = true
	}
	return nil
}
