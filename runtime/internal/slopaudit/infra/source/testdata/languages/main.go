package main

import "fmt"

func Work() {
	_ = risky()
	fmt.Println("debug temporary")
	panic("TODO not implemented")
}
func Empty()       {}
func risky() error { return nil }
func Swallow() error {
	_, err := fmt.Println("x")
	if err != nil {
	}
	return nil
}
