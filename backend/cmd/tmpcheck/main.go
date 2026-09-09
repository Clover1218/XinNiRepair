package main

import (
	"context"
	"fmt"

	"xin-ni-repair/internal/config"
	"xin-ni-repair/internal/model"
	"xin-ni-repair/internal/repository"
	"xin-ni-repair/internal/service"
)

func main() {
	cfg, _ := config.Load("config/config.yaml")
	db, err := repository.New(context.Background(), cfg.Database)
	if err != nil {
		panic(err)
	}
	var u model.User
	if err := db.DB.Where("role = ?", 2).First(&u).Error; err != nil {
		panic(err)
	}
	ts := service.NewTokenService(cfg.JWT)
	token, _, _ := ts.GenerateAccessToken(u.ID, u.Role)
	fmt.Print(token)
}
