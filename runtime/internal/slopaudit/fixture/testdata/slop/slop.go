package slop

import "fmt"

// TODO finish for real
func Empty() {}

func Work() {
	_ = risky()
	fmt.Println("debug temporary")
	panic("TODO not implemented")
}

func risky() error { return nil }
