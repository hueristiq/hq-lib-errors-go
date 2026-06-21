package main

import (
	"fmt"

	hqgoerrors "github.com/hueristiq/hq-lib-errors-go"
)

func main() {
	err1 := hqgoerrors.New("error 1")
	err2 := hqgoerrors.New("error 2")
	err3 := fmt.Errorf("error 3")

	err := hqgoerrors.Join(err1, err2, err3)

	formattedStr := hqgoerrors.ToString(err, hqgoerrors.FormatWithTrace())

	fmt.Println(formattedStr)
}
