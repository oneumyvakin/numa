# NUMA

NUMA is a library to get basic NUMA nodes information from Linux system.

```go
package main

import (
	"fmt"
	"os"
	
	"github.com/oneumyvakin/numa"
)

func main() {
	nodes, err := numa.GetNodes()

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("%#v\n", nodes)
}
```
