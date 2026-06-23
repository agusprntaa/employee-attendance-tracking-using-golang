package main

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "123456"

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 6)
	if err != nil {
		panic(err)
	}

	fmt.Println("HASH:", string(hash))
}
