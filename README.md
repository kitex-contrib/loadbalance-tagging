# Tagging Loadbalance

Divide the cluster into different subsets based on tags on the client, suitable for stateful services or multi-tenant services

## How to use?

Client

```go
import (
	...
	"github.com/cloudwego/kitex/client"
	tagging "github.com/kitex-contrib/loadbalance-tagging"
	...
)

func main() {
	...
	// create a LoadBalancer with loadbalance 
	client, err := client.NewClient("echo", client.WithLoadBalancer(tagging.New(tag, tagFunc, nextLoadBalancer)))
	if err != nil {
		log.Fatal(err)
	}
	...
}
```

Multi-tag selector can be implemented by combining tag selectors in sequence
```go
import (
	...
	"github.com/cloudwego/kitex/client"
	tagging "github.com/kitex-contrib/loadbalance-tagging"
	...
)

func main() {
	... 
	// create a LoadBalancer with multi-tag selectors 
	client, err := client.NewClient("echo", 
		client.WithLoadBalancer(tagging.New(tag1, tag1Func, tagging.New(tag2, tag2Func, nextLoadBalancer))))
	if err != nil {
		log.Fatal(err)
	}
	...
}
```

## Maintainer

maintained by: [jizhuozhi (jizhuozhi.george@gmail.com)](https://github.com/jizhuozhi)