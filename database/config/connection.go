package config

import "github.com/upper/db/v4/adapter/postgresql"

func NewConfiguration(host, name, user, password string, options map[string]string) postgresql.ConnectionURL {
	return postgresql.ConnectionURL{
		Host:     host,
		User:     user,
		Password: password,
		Database: name,
		Options:  options,
	}
}
