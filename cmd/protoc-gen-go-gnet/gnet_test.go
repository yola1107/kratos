package main

import (
	"strings"
	"testing"
)

func TestServiceDesc(t *testing.T) {
	sd := &serviceDesc{
		ServiceType: "Greeter",
		ServiceName: "helloworld.Greeter",
		Metadata:    "api/helloworld/helloworld.proto",
		Methods: []*methodDesc{
			{
				Name:         "SayHello",
				OriginalName: "SayHello",
				Num:          0,
				Request:      "HelloRequest",
				Reply:        "HelloReply",
				Comment:      "",
				Ops:          "1",
			},
		},
	}
	result := sd.execute()
	if result == "" {
		t.Fatal("execute() should not return empty string")
	}
	if !strings.Contains(result, "GreeterGNETServer") {
		t.Fatal("result should contain GreeterGNETServer")
	}
	if !strings.Contains(result, "RegisterGreeterGNETServer") {
		t.Fatal("result should contain RegisterGreeterGNETServer")
	}
}
