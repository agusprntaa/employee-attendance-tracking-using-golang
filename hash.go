package main

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func hash() {
	hash, _ := bcrypt.GenerateFromPassword([]byte("12345678"), 10)
	fmt.Println(string(hash))
}