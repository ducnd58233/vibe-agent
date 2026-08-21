package pkg_b

import "example.com/fixture/pkg_a"

func Use() string {
	return pkg_a.Helper()
}
