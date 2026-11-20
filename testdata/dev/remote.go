package main

import "fmt"

// greet builds a greeting
func greet(name string) string {
	return "Hey " + name + "!"
}

func main() {
	fmt.Println(greet("Go"))
}
