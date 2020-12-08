package main

import (
	"fmt"
	"regexp"
)

func main() {
	r, err := regexp.Compile(`http://scp-jp\.wikidot\.com/scp-[0-9]{1,}-jp`)
	if err != nil {
		fmt.Println("regex comple error:")
		fmt.Println(err)
	}

	fmt.Println(r.MatchString("http://scp-jp.wikidot.com/scp-001-jp"))
	fmt.Println(r.MatchString("http://www.wikidot.com"))
}
