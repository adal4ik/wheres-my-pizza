package config

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

type DB struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	User string `yaml:"user"`
	Pass string `yaml:"password"`
	Name string `yaml:"database"`
}

type MQ struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	User string `yaml:"user"`
	Pass string `yaml:"password"`
}

type App struct {
	Database DB `yaml:"database"`
	Rabbit   MQ `yaml:"rabbitmq"`
}

// простой YAML-парсер без внешних пакетов (ожидает 2 уровня вложенности)
func Load(path string) (App, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return App{}, err
	}
	var a App
	var cur string
	for _, ln := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(ln)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(line, ":") {
			cur = strings.TrimSuffix(line, ":")
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.Trim(strings.TrimSpace(parts[1]), "\"")
		switch cur {
		case "database":
			assignDB(&a.Database, k, v)
		case "rabbitmq":
			assignMQ(&a.Rabbit, k, v)
		}
	}
	if a.Database.Host == "" || a.Rabbit.Host == "" {
		return App{}, errors.New("invalid config: missing database/rabbitmq host")
	}
	return a, nil
}

func assignDB(d *DB, k, v string) {
	switch k {
	case "host":
		d.Host = v
	case "port":
		d.Port = atoiSafe(v)
	case "user":
		d.User = v
	case "password":
		d.Pass = v
	case "database":
		d.Name = v
	}
}

func assignMQ(m *MQ, k, v string) {
	switch k {
	case "host":
		m.Host = v
	case "port":
		m.Port = atoiSafe(v)
	case "user":
		m.User = v
	case "password":
		m.Pass = v
	}
}

func atoiSafe(s string) int { var n int; for _, r := range s { if r >= '0' && r <= '9' { n = n*10 + int(r-'0') } }; return n }

func FindConfig() (string, error) {
	candidates := []string{"config.yaml", "deploy/config.example.yaml"}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fs.ErrNotExist
}
