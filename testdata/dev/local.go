package main

import "fmt"

// greet returns a greeting
func greet(name string) string {
	return "Hi, " + name
}

func main() {
	fmt.Println(greet("world"))
}
