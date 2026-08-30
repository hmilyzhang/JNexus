package main

import (
	"fmt"

	"autoops/internal/config"
	"autoops/internal/model"
	"autoops/internal/service"
)

func main() {
	config.Cfg.Database.DSN = "host=127.0.0.1 port=5432 user=autoops password=autoops123 dbname=autoops sslmode=disable"
	config.Cfg.Auth.AESKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	if err := model.Connect(config.Cfg.Database.DSN); err != nil {
		panic(err)
	}
	s := service.LoadLDAPSettings()
	fmt.Printf("required=%#v\n", s.RequiredGroups)
	_, _, err := service.LDAPLogin(s, "bob", "bob123")
	fmt.Println("bob err:", err)
	_, _, err = service.LDAPLogin(s, "alice", "alice123")
	fmt.Println("alice err:", err)
}
