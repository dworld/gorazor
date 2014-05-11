package gorazor

import (
	"fmt"
	_ "testing"
)

func ExampleCapitalize() {
	fmt.Println(Capitalize("hello"))
	fmt.Println(Capitalize("he"))
	fmt.Println(Capitalize("h"))
	fmt.Println(Capitalize(""))
	// Output:
	// Hello
	// He
	// H
	//
}
